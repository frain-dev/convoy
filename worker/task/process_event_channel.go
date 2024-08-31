package task

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/util"
	"github.com/hibiken/asynq"
	"strings"
	"time"
)

type EventChannelConfig struct {
	Channel      string
	DefaultDelay time.Duration
}
type EventChannelMetadata struct {
	Event  *datastore.Event
	Config *EventChannelConfig
}

type EventChannelSubResponse struct {
	Event            *datastore.Event
	Project          *datastore.Project
	Subscriptions    []datastore.Subscription
	IsDuplicateEvent bool
}

type EventChannel interface {
	GetConfig() *EventChannelConfig
	CreateEvent(context.Context, *asynq.Task, EventChannel, datastore.EventRepository, datastore.ProjectRepository, datastore.EndpointRepository, datastore.SubscriptionRepository, license.Licenser) (*datastore.Event, error)
	MatchSubscriptions(context.Context, EventChannelMetadata, datastore.EventRepository, datastore.ProjectRepository, datastore.EndpointRepository, datastore.SubscriptionRepository, license.Licenser) (*EventChannelSubResponse, error)
}

func ProcessEventCreationByChannel(channel EventChannel, endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, licenser license.Licenser) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		cfg := channel.GetConfig()

		// get or create event
		var lastEvent, lastRunErrored, err = getLastTaskInfo(ctx, t, channel, eventQueue, eventRepo)
		if lastEvent != nil && lastEvent.IsDuplicateEvent && !lastRunErrored {
			log.FromContext(ctx).Infof("[asynq]: duplicate event with idempotency key %v will not be sent", lastEvent.IdempotencyKey)
			return nil
		}

		var event *datastore.Event
		if lastEvent != nil {
			log.Info("[asynq] processing last event")
			event = lastEvent
		} else {
			log.Info("[asyq] creating new event")
			event, err = channel.CreateEvent(ctx, t, channel, eventRepo, projectRepo, endpointRepo, subRepo, licenser)
			if err != nil {
				if strings.Contains(err.Error(), "duplicate key") {
					lastEvent, err = eventRepo.FindEventByID(ctx, event.ProjectID, event.UID)
				}

				if lastEvent == nil {
					writeErr := fmt.Errorf("failed to create event, err: %s", err.Error())
					err = &EndpointError{Err: writeErr, delay: cfg.DefaultDelay}
					return err
				}

				log.WithError(err).Error("skipping duplicated event: " + event.UID)
				return nil
			}
			if event == nil {
				return &EndpointError{Err: fmt.Errorf("CODE: 1009, no response, failed to create event via channel %s", cfg.Channel), delay: cfg.DefaultDelay}
			}
		}

		metadata := EventChannelMetadata{
			Event:  event,
			Config: cfg,
		}

		payload, err := msgpack.EncodeMsgPack(metadata)
		if err != nil {
			return err
		}

		jobId := fmt.Sprintf("match_subs:%s:%s", event.ProjectID, event.UID)
		job := &queue.Job{
			ID:      jobId,
			Payload: payload,
			Delay:   0,
		}

		log.Info("[asynq] writing to queue to match subscriptions")
		err = eventQueue.Write(convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, job)
		if err != nil {
			log.FromContext(ctx).WithError(err).Errorf("[asynq]: an error occurred while matching event subs")
		}

		return err
	}
}

func MatchSubscriptionsAndCreateEventDeliveries(channels map[string]EventChannel, endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, deviceRepo datastore.DeviceRepository, licenser license.Licenser) func(context.Context, *asynq.Task) error {
	return func(ctx0 context.Context, t *asynq.Task) error {

		ctx := context.WithoutCancel(ctx0)

		log.Errorln("matching subscriptions")
		var metadata EventChannelMetadata
		err := getTaskPayload(t, &metadata)
		if err != nil {
			return err
		}

		channel := channels[metadata.Config.Channel]
		if channel == nil {
			log.Errorf("Invalid channel %s\n", metadata.Config.Channel)
			return nil
		}
		cfg := metadata.Config
		log.Errorf("about to match subs for channel: %s\n", cfg.Channel)

		subResponse, err := channel.MatchSubscriptions(ctx, metadata, eventRepo, projectRepo, endpointRepo, subRepo, licenser)
		if err != nil {
			return err
		}
		if subResponse == nil {
			return &EndpointError{Err: fmt.Errorf("CODE: 1009, failed to create event subscriptions via channel: %s", cfg.Channel), delay: cfg.DefaultDelay}
		}

		event, subscriptions := subResponse.Event, subResponse.Subscriptions
		if len(subscriptions) < 1 {
			err = &EndpointError{Err: fmt.Errorf("CODE: 1010, empty subscriptions via channel %s", cfg.Channel), delay: cfg.DefaultDelay}
			log.WithError(err).Errorf("failed to send %s", event.UID)
			return eventRepo.UpdateEventStatus(ctx, event, datastore.FailureStatus)
		}
		//if event.Endpoints == nil {
		//	if len(event.Endpoints) < 1 {
		var endpointIDs []string
		for _, s := range subscriptions {
			if s.Type != datastore.SubscriptionTypeCLI {
				endpointIDs = append(endpointIDs, s.EndpointID)
			}
		}
		event.Endpoints = endpointIDs
		//}

		err = eventRepo.UpdateEventEndpoints(ctx, event, event.Endpoints)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}
		//}

		if subResponse.IsDuplicateEvent {
			log.FromContext(ctx).Infof("[asynq]: duplicate event with idempotency key %v will not be sent", event.IdempotencyKey)
			return nil
		}

		// no need for a separate queue
		err = writeEventDeliveriesToQueue(ctx, subResponse.Subscriptions, subResponse.Event, subResponse.Project, eventDeliveryRepo, eventQueue, deviceRepo, endpointRepo, licenser)
		if err != nil {
			log.WithError(err).Error(ErrFailedToWriteToQueue)
			writeErr := fmt.Errorf("%s, err: %s", ErrFailedToWriteToQueue.Error(), err.Error())
			err = &EndpointError{Err: writeErr, delay: cfg.DefaultDelay}
			return err
		}

		err = eventRepo.UpdateEventStatus(ctx, event, datastore.SuccessStatus)
		if err != nil {
			log.WithError(err).Errorf("failed to update event status: %s", event.UID)
		}

		return err
	}
}

func getLastTaskInfo(ctx context.Context, t *asynq.Task, ch EventChannel, eventQueue queue.Queuer, eventRepo datastore.EventRepository) (*datastore.Event, bool, error) {
	var jobID string
	switch ch.GetConfig().Channel {
	case "broadcast":
		var broadcastEvent models.BroadcastEvent
		err := getTaskPayload(t, &broadcastEvent)
		if err != nil {
			return nil, false, err
		}
		jobID = broadcastEvent.JobID

	case "default":
		var createEvent CreateEvent
		err := getTaskPayload(t, &createEvent)
		if err != nil {
			return nil, false, err
		}
		jobID = createEvent.JobID

	case "dynamic":
		var dynamicEvent models.DynamicEvent
		err := getTaskPayload(t, &dynamicEvent)
		if err != nil {
			return nil, false, err
		}
		jobID = dynamicEvent.JobID
	}
	if util.IsStringEmpty(jobID) || !strings.Contains(jobID, ":") {
		return nil, false, &EndpointError{Err: fmt.Errorf("cannot deduce jobID: %s", jobID)}
	}

	q := eventQueue.(*redis.RedisQueue)
	ti, err := q.Inspector().GetTaskInfo(string(convoy.CreateEventQueue), jobID)
	if err != nil {
		log.WithError(err).Error("failed to get task from queue")
		return nil, false, &EndpointError{Err: fmt.Errorf("failed to get task from queue, err: %s", err.Error()), delay: defaultBroadcastDelay}
	}

	lastRunErrored := ti != nil && strings.Contains(ti.LastErr, ErrFailedToWriteToQueue.Error())

	var lastEvent *datastore.Event

	if lastRunErrored {
		split := strings.Split(ti.LastErr, ":")
		if len(split) == 3 {
			projectId, eventId := split[1], split[2]
			if !util.IsStringEmpty(projectId) && !util.IsStringEmpty(eventId) {
				lastEvent, err = eventRepo.FindEventByID(ctx, projectId, eventId)
			}
		}
	}
	return lastEvent, lastRunErrored, err
}

func getTaskPayload(t *asynq.Task, pojo interface{}) error {
	err := msgpack.DecodeMsgPack(t.Payload(), &pojo)
	if err != nil {
		err := json.Unmarshal(t.Payload(), &pojo)
		if err != nil {
			return err
		}
	}
	return err
}
