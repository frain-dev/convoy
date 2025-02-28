package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/util"
	"github.com/hibiken/asynq"
)

type EventChannelConfig struct {
	Channel      string
	DefaultDelay time.Duration
}
type EventChannelMetadata struct {
	Event  *datastore.Event
	Config *EventChannelConfig
}

type EventChannelArgs struct {
	eventRepo     datastore.EventRepository
	projectRepo   datastore.ProjectRepository
	endpointRepo  datastore.EndpointRepository
	subRepo       datastore.SubscriptionRepository
	filterRepo    datastore.FilterRepository
	licenser      license.Licenser
	tracerBackend tracer.Backend
}

type EventChannelSubResponse struct {
	Event            *datastore.Event
	Project          *datastore.Project
	Subscriptions    []datastore.Subscription
	IsDuplicateEvent bool
}

type EventChannel interface {
	GetConfig() *EventChannelConfig
	CreateEvent(context.Context, *asynq.Task, EventChannel, EventChannelArgs) (*datastore.Event, error)
	MatchSubscriptions(context.Context, EventChannelMetadata, EventChannelArgs) (*EventChannelSubResponse, error)
}

func ProcessEventCreationByChannel(channel EventChannel, endpointRepo datastore.EndpointRepository,
	eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository,
	eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, filterRepo datastore.FilterRepository,
	licenser license.Licenser, tracerBackend tracer.Backend) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		cfg := channel.GetConfig()

		// get or create event
		var lastEvent, lastRunErrored, err = getLastTaskInfo(ctx, t, channel, eventQueue, eventRepo)
		if lastEvent != nil && lastEvent.IsDuplicateEvent && !lastRunErrored {
			log.FromContext(ctx).Debugf("[asynq]: duplicate event with idempotency key %v will not be sent", lastEvent.IdempotencyKey)
			return nil
		}

		var event *datastore.Event
		if lastEvent != nil {
			event = lastEvent
		} else {
			event, err = channel.CreateEvent(ctx, t, channel, EventChannelArgs{
				eventRepo:     eventRepo,
				projectRepo:   projectRepo,
				endpointRepo:  endpointRepo,
				subRepo:       subRepo,
				filterRepo:    filterRepo,
				licenser:      licenser,
				tracerBackend: tracerBackend,
			})
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

		err = eventQueue.Write(convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, job)
		if err != nil {
			log.FromContext(ctx).WithError(err).Errorf("[asynq]: an error occurred while matching event subs")
		}

		return err
	}
}

func MatchSubscriptionsAndCreateEventDeliveries(channels map[string]EventChannel, endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer, subRepo datastore.SubscriptionRepository, filterRepo datastore.FilterRepository, deviceRepo datastore.DeviceRepository, licenser license.Licenser, tracerBackend tracer.Backend) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		// Start a new trace span for subscription matching and event delivery creation
		startTime := time.Now()
		attributes := map[string]interface{}{
			"event.type": "event.subscription.matching",
		}

		var metadata EventChannelMetadata
		err := getTaskPayload(t, &metadata)
		if err != nil {
			tracerBackend.Capture(ctx, "event.subscription.matching.error", attributes, startTime, time.Now())
			return err
		}

		attributes["event.id"] = metadata.Event.UID

		channel := channels[metadata.Config.Channel]
		if channel == nil {
			log.Errorf("Invalid channel %s\n", metadata.Config.Channel)
			tracerBackend.Capture(ctx, "event.subscription.matching.error", attributes, startTime, time.Now())
			return nil
		}

		attributes["channel"] = metadata.Config.Channel
		cfg := metadata.Config
		log.Infof("about to match subs for channel: %s\n", cfg.Channel)

		subResponse, err := channel.MatchSubscriptions(ctx, metadata, EventChannelArgs{
			eventRepo:     eventRepo,
			projectRepo:   projectRepo,
			endpointRepo:  endpointRepo,
			subRepo:       subRepo,
			filterRepo:    filterRepo,
			licenser:      licenser,
			tracerBackend: tracerBackend,
		})
		if err != nil {
			tracerBackend.Capture(ctx, "event.subscription.matching.error", attributes, startTime, time.Now())
			return err
		}
		if subResponse == nil {
			tracerBackend.Capture(ctx, "event.subscription.matching.error", attributes, startTime, time.Now())
			return &EndpointError{Err: fmt.Errorf("CODE: 1010, failed to create event subscriptions via channel: %s", cfg.Channel), delay: cfg.DefaultDelay}
		}

		attributes["project.id"] = subResponse.Project.UID

		event, subscriptions := subResponse.Event, subResponse.Subscriptions
		if len(subscriptions) < 1 {
			err = &EndpointError{Err: fmt.Errorf("CODE: 1011, empty subscriptions via channel %s", cfg.Channel), delay: cfg.DefaultDelay}
			log.WithError(err).Errorf("failed to send %s", event.UID)
			tracerBackend.Capture(ctx, "event.subscription.matching.error", attributes, startTime, time.Now())
			return eventRepo.UpdateEventStatus(ctx, event, datastore.FailureStatus)
		}

		var endpointIDs []string
		for _, s := range subscriptions {
			if s.Type != datastore.SubscriptionTypeCLI {
				endpointIDs = append(endpointIDs, s.EndpointID)
			}
		}
		event.Endpoints = endpointIDs

		err = eventRepo.UpdateEventEndpoints(ctx, event, event.Endpoints)
		if err != nil {
			tracerBackend.Capture(ctx, "event.subscription.matching.error", attributes, startTime, time.Now())
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		if subResponse.IsDuplicateEvent {
			log.FromContext(ctx).Infof("CODE: 1007, duplicate event with idempotency key %v will not be sent", event.IdempotencyKey)
			tracerBackend.Capture(ctx, "event.subscription.matching.duplicate", attributes, startTime, time.Now())
			return nil
		}

		// no need for a separate queue
		err = writeEventDeliveriesToQueue(ctx, subResponse.Subscriptions, subResponse.Event, subResponse.Project, eventDeliveryRepo, eventQueue, deviceRepo, endpointRepo, licenser)
		if err != nil {
			log.WithError(err).Error(ErrFailedToWriteToQueue)
			writeErr := fmt.Errorf("%s, err: %s", ErrFailedToWriteToQueue.Error(), err.Error())
			err = &EndpointError{Err: writeErr, delay: cfg.DefaultDelay}
			_ = eventRepo.UpdateEventStatus(ctx, event, datastore.RetryStatus)
			tracerBackend.Capture(ctx, "event.subscription.matching.error", attributes, startTime, time.Now())
			return err
		}

		err = eventRepo.UpdateEventStatus(ctx, event, datastore.SuccessStatus)
		if err != nil {
			log.WithError(err).Errorf("failed to update event status: %s", event.UID)
			tracerBackend.Capture(ctx, "event.subscription.matching.error", attributes, startTime, time.Now())
			return err
		}

		tracerBackend.Capture(ctx, "event.subscription.matching.success", attributes, startTime, time.Now())
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
