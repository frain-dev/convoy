package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/dynamiceventack"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

// reasonNoMatchingSubscriptions is shown against a failed event in the
// dashboard. It is deliberately static operator facing text, so no event
// payload, URL, or credential can reach a user visible field.
const reasonNoMatchingSubscriptions = "no subscription matched this event"

// reasonMissingEndpointID is shown against a failed event in the dashboard.
// Failure policy: a NOT NULL insert with a null endpoint_id is a deterministic
// validation failure. Persist Failure and return nil so the worker completes
// instead of retrying forever.
const reasonMissingEndpointID = "subscription matched without an endpoint_id"

type TaskErrorReader interface {
	LastTaskError(queueName, jobID string) (string, error)
}

type EventChannelConfig struct {
	Channel      string
	DefaultDelay time.Duration
}
type EventChannelMetadata struct {
	Event  *datastore.Event
	Config *EventChannelConfig
}

type EventChannelArgs struct {
	eventRepo                  datastore.EventRepository
	projectRepo                datastore.ProjectRepository
	endpointRepo               datastore.EndpointRepository
	subRepo                    datastore.SubscriptionRepository
	filterRepo                 datastore.FilterRepository
	licenser                   license.Licenser
	oauth2TokenService         OAuth2TokenService
	featureFlag                *fflag.FFlag
	featureFlagFetcher         fflag.FeatureFlagFetcher
	earlyAdopterFeatureFetcher fflag.EarlyAdopterFeatureFetcher
	acker                      dynamiceventack.Acker
	logger                     log.Logger
	taskRetryCount             int
}

const headerRetryCount = "X-Convoy-Retry-Count"

// matchTaskRetryCount reads how many times this queue job has already run.
// Postgres workers stamp X-Convoy-Retry-Count; Redis asynq workers use ctx.
func matchTaskRetryCount(ctx context.Context, t *asynq.Task) int {
	if t != nil {
		if headers := t.Headers(); headers != nil {
			if v, ok := headers[headerRetryCount]; ok {
				if n, err := strconv.Atoi(v); err == nil {
					return n
				}
			}
		}
	}
	if n, ok := asynq.GetRetryCount(ctx); ok {
		return n
	}
	return 0
}

// eventForMatch returns the event row MatchSubscriptions should use.
// Failure policy: first attempt trusts the create/match payload snapshot; retries
// reload from DB so status and idempotency reflect partial match work.
func eventForMatch(ctx context.Context, repo datastore.EventRepository, metadata EventChannelMetadata, taskRetryCount int) (*datastore.Event, error) {
	if metadata.Event == nil || util.IsStringEmpty(metadata.Event.UID) {
		return nil, fmt.Errorf("missing event in match metadata")
	}
	if taskRetryCount == 0 {
		return metadata.Event, nil
	}
	return repo.FindEventByID(ctx, metadata.Event.ProjectID, metadata.Event.UID)
}

type EventChannelSubResponse struct {
	Event            *datastore.Event
	Project          *datastore.Project
	Subscriptions    []datastore.Subscription
	IsDuplicateEvent bool
	TargetURL        string
}

type EventChannel interface {
	GetConfig() *EventChannelConfig
	CreateEvent(context.Context, *asynq.Task, EventChannel, EventChannelArgs) (*datastore.Event, error)
	MatchSubscriptions(context.Context, EventChannelMetadata, EventChannelArgs) (*EventChannelSubResponse, error)
}

func ProcessEventCreationByChannel(channel EventChannel, endpointRepo datastore.EndpointRepository,
	eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository,
	eventQueue queue.Queuer, taskErrors TaskErrorReader, subRepo datastore.SubscriptionRepository, filterRepo datastore.FilterRepository,
	licenser license.Licenser, oauth2TokenService OAuth2TokenService, featureFlag *fflag.FFlag,
	featureFlagFetcher fflag.FeatureFlagFetcher, earlyAdopterFeatureFetcher fflag.EarlyAdopterFeatureFetcher,
	acker dynamiceventack.Acker, logger log.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		cfg := channel.GetConfig()

		// get or create event
		var lastEvent, _, err = getLastTaskInfo(ctx, t, channel, taskErrors, eventRepo, logger)
		if err != nil {
			logger.Error("failed to get last task info", "error", err)
			return err
		}

		// Note: getLastTaskInfo only loads lastEvent when lastRunErrored is true, so
		// an early "duplicate + !lastRunErrored" short-circuit is unreachable.

		var event *datastore.Event
		if lastEvent != nil {
			event = lastEvent
		} else {
			event, err = channel.CreateEvent(ctx, t, channel, EventChannelArgs{
				eventRepo:                  eventRepo,
				projectRepo:                projectRepo,
				endpointRepo:               endpointRepo,
				subRepo:                    subRepo,
				filterRepo:                 filterRepo,
				licenser:                   licenser,
				oauth2TokenService:         oauth2TokenService,
				featureFlag:                featureFlag,
				featureFlagFetcher:         featureFlagFetcher,
				earlyAdopterFeatureFetcher: earlyAdopterFeatureFetcher,
				acker:                      acker,
				logger:                     logger,
			})
			if err != nil {
				createErr := err
				if event != nil && strings.Contains(createErr.Error(), "duplicate key") {
					// Heal incomplete creates: row exists but match may not have run.
					// Failure policy: rematch only when status is still Pending (or Retry).
					// Success/Processing must not re-Write match — Asynq deletes+requeues
					// the same task ID and can fan out deliveries again while match runs.
					found, findErr := eventRepo.FindEventByID(ctx, event.ProjectID, event.UID)
					if findErr != nil || found == nil {
						writeErr := fmt.Errorf("failed to create event, err: %s", createErr.Error())
						return &EndpointError{Err: writeErr, delay: cfg.DefaultDelay}
					}
					event = found
					switch found.Status {
					case datastore.SuccessStatus:
						if cfg.Channel == "dynamic" {
							publishDynamicEventAck(ctx, acker, logger, found.ProjectID, found.UID, dynamiceventack.Result{OK: true})
						}
						return nil
					case datastore.ProcessingStatus, datastore.FailureStatus:
						return nil
					}
					logger.Error("duplicate event create; continuing to match: "+event.UID, "error", createErr)
				} else {
					writeErr := fmt.Errorf("failed to create event, err: %s", createErr.Error())
					return &EndpointError{Err: writeErr, delay: cfg.DefaultDelay}
				}
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

		jobId := queue.JobId{ProjectID: event.ProjectID, ResourceID: event.UID}.MatchSubsJobId()
		job := &queue.Job{
			ID:      jobId,
			Payload: payload,
			Delay:   0,
		}

		err = eventQueue.Write(ctx, convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, job)
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("[asynq]: an error occurred while matching event subs: %v", err))
		}

		return err
	}
}

type MatchSubscriptionsDeps struct {
	Channels                   map[string]EventChannel
	EndpointRepo               datastore.EndpointRepository
	EventRepo                  datastore.EventRepository
	ProjectRepo                datastore.ProjectRepository
	EventDeliveryRepo          datastore.EventDeliveryRepository
	EventQueue                 queue.Queuer
	SubRepo                    datastore.SubscriptionRepository
	FilterRepo                 datastore.FilterRepository
	Licenser                   license.Licenser
	OAuth2TokenService         OAuth2TokenService
	FeatureFlag                *fflag.FFlag
	FeatureFlagFetcher         fflag.FeatureFlagFetcher
	EarlyAdopterFeatureFetcher fflag.EarlyAdopterFeatureFetcher
	Acker                      dynamiceventack.Acker
	Logger                     log.Logger
}

func MatchSubscriptionsAndCreateEventDeliveries(deps MatchSubscriptionsDeps) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		// Start a new trace span for subscription matching and event delivery creation
		attributes := map[string]interface{}{
			"event.type": "event.subscription.matching",
		}

		var metadata EventChannelMetadata
		err := getTaskPayload(t, &metadata)
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventEventSubscriptionMatchingError, attributes)
			return err
		}

		attributes["event.id"] = metadata.Event.UID

		channel := deps.Channels[metadata.Config.Channel]
		if channel == nil {
			deps.Logger.Error(fmt.Sprintf("Invalid channel %s\n", metadata.Config.Channel))
			tracer.AddEvent(ctx, tracer.EventEventSubscriptionMatchingError, attributes)
			return nil
		}

		attributes["channel"] = metadata.Config.Channel
		cfg := metadata.Config
		deps.Logger.Info(fmt.Sprintf("about to match subs for channel: %s\n", cfg.Channel))

		subResponse, err := channel.MatchSubscriptions(ctx, metadata, EventChannelArgs{
			eventRepo:                  deps.EventRepo,
			projectRepo:                deps.ProjectRepo,
			endpointRepo:               deps.EndpointRepo,
			subRepo:                    deps.SubRepo,
			filterRepo:                 deps.FilterRepo,
			licenser:                   deps.Licenser,
			oauth2TokenService:         deps.OAuth2TokenService,
			featureFlag:                deps.FeatureFlag,
			featureFlagFetcher:         deps.FeatureFlagFetcher,
			earlyAdopterFeatureFetcher: deps.EarlyAdopterFeatureFetcher,
			acker:                      deps.Acker,
			logger:                     deps.Logger,
			taskRetryCount:             matchTaskRetryCount(ctx, t),
		})
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventEventSubscriptionMatchingError, attributes)
			return err
		}
		if subResponse == nil {
			tracer.AddEvent(ctx, tracer.EventEventSubscriptionMatchingError, attributes)
			return &EndpointError{Err: fmt.Errorf("CODE: 1010, failed to create event subscriptions via channel: %s", cfg.Channel), delay: cfg.DefaultDelay}
		}

		attributes["project.id"] = subResponse.Project.UID

		event, subscriptions := subResponse.Event, subResponse.Subscriptions
		if subResponse.IsDuplicateEvent {
			deps.Logger.InfoContext(ctx, fmt.Sprintf("CODE: 1007, duplicate event with idempotency key %v will not be sent", event.IdempotencyKey))
			tracer.AddEvent(ctx, tracer.EventEventSubscriptionMatchingDuplicate, attributes)
			return nil
		}

		if len(subscriptions) < 1 {
			err = &EndpointError{Err: fmt.Errorf("CODE: 1011, empty subscriptions via channel %s", cfg.Channel), delay: cfg.DefaultDelay}
			deps.Logger.Error(fmt.Sprintf("failed to send %s: %v", event.UID, err))
			tracer.AddEvent(ctx, tracer.EventEventSubscriptionMatchingError, attributes)
			return deps.EventRepo.UpdateEventStatus(ctx, event, datastore.FailureStatus, reasonNoMatchingSubscriptions)
		}

		endpointIDs, err := collectAPIEndpointIDs(subscriptions)
		if err != nil {
			deps.Logger.Error(fmt.Sprintf("failed to send %s: %v", event.UID, err))
			tracer.AddEvent(ctx, tracer.EventEventSubscriptionMatchingError, attributes)
			return deps.EventRepo.UpdateEventStatus(ctx, event, datastore.FailureStatus, reasonMissingEndpointID)
		}
		event.Endpoints = endpointIDs

		err = deps.EventRepo.UpdateEventEndpoints(ctx, event, event.Endpoints)
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventEventSubscriptionMatchingError, attributes)
			if errors.Is(err, datastore.ErrEventEndpointIDRequired) {
				deps.Logger.Error(fmt.Sprintf("failed to send %s: %v", event.UID, err))
				return deps.EventRepo.UpdateEventStatus(ctx, event, datastore.FailureStatus, reasonMissingEndpointID)
			}
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		// no need for a separate queue
		err = writeEventDeliveriesToQueue(ctx, WriteEventDeliveriesToQueueOptions{
			Subscriptions:              subResponse.Subscriptions,
			Event:                      subResponse.Event,
			Project:                    subResponse.Project,
			TargetURL:                  subResponse.TargetURL,
			EventDeliveryRepo:          deps.EventDeliveryRepo,
			EventQueue:                 deps.EventQueue,
			EndpointRepo:               deps.EndpointRepo,
			Licenser:                   deps.Licenser,
			OAuth2TokenService:         deps.OAuth2TokenService,
			FeatureFlag:                deps.FeatureFlag,
			FeatureFlagFetcher:         deps.FeatureFlagFetcher,
			EarlyAdopterFeatureFetcher: deps.EarlyAdopterFeatureFetcher,
			Logger:                     deps.Logger,
		})
		if err != nil {
			deps.Logger.Error(ErrFailedToWriteToQueue.Error(), "error", err)
			writeErr := fmt.Errorf("%s, err: %s", ErrFailedToWriteToQueue.Error(), err.Error())
			err = &EndpointError{Err: writeErr, delay: cfg.DefaultDelay}
			_ = deps.EventRepo.UpdateEventStatus(ctx, event, datastore.RetryStatus, "")
			tracer.AddEvent(ctx, tracer.EventEventSubscriptionMatchingError, attributes)
			return err
		}

		err = deps.EventRepo.UpdateEventStatus(ctx, event, datastore.SuccessStatus, "")
		if err != nil {
			deps.Logger.Error(fmt.Sprintf("failed to update event status: %s: %v", event.UID, err))
			tracer.AddEvent(ctx, tracer.EventEventSubscriptionMatchingError, attributes)
			return err
		}

		tracer.AddEvent(ctx, tracer.EventEventSubscriptionMatchingSuccess, attributes)
		return err
	}
}

func collectAPIEndpointIDs(subscriptions []datastore.Subscription) ([]string, error) {
	ids := make([]string, 0, len(subscriptions))
	for i := range subscriptions {
		s := &subscriptions[i]
		if s.Type == datastore.SubscriptionTypeCLI {
			continue
		}
		if util.IsStringEmpty(s.EndpointID) {
			return nil, datastore.ErrEventEndpointIDRequired
		}
		ids = append(ids, s.EndpointID)
	}
	return ids, nil
}

func getLastTaskInfo(ctx context.Context, t *asynq.Task, ch EventChannel, taskErrors TaskErrorReader, eventRepo datastore.EventRepository, logger log.Logger) (*datastore.Event, bool, error) {
	var jobID string
	switch ch.GetConfig().Channel {
	case "broadcast":
		var broadcastEvent models.BroadcastEvent
		err := getTaskPayload(t, &broadcastEvent)
		if err != nil {
			return nil, false, err
		}
		jobID = broadcastEvent.JobID
	case "dynamic":
		var dynamicEvent models.DynamicEvent
		err := getTaskPayload(t, &dynamicEvent)
		if err != nil {
			return nil, false, err
		}
		jobID = dynamicEvent.JobID
	default:
		var createEvent CreateEvent
		err := getTaskPayload(t, &createEvent)
		if err != nil {
			return nil, false, err
		}
		jobID = createEvent.JobID
	}

	if util.IsStringEmpty(jobID) || !strings.Contains(jobID, ":") {
		return nil, false, &EndpointError{Err: fmt.Errorf("cannot deduce jobID: %s", jobID)}
	}

	if taskErrors == nil {
		return nil, false, nil
	}

	lastTaskError, err := taskErrors.LastTaskError(string(convoy.CreateEventQueue), jobID)
	if err != nil {
		logger.Error("failed to get task from queue", "error", err)
		return nil, false, &EndpointError{Err: fmt.Errorf("failed to get task from queue, err: %s", err.Error()), delay: defaultBroadcastDelay}
	}

	lastRunErrored := strings.Contains(lastTaskError, ErrFailedToWriteToQueue.Error())

	var lastEvent *datastore.Event

	if lastRunErrored {
		split := strings.Split(lastTaskError, ":")
		if len(split) == 3 {
			projectId, eventId := split[1], split[2]
			if !util.IsStringEmpty(projectId) && !util.IsStringEmpty(eventId) {
				lastEvent, err = eventRepo.FindEventByID(ctx, projectId, eventId)
			}
		}
	}
	return lastEvent, lastRunErrored, err
}

func getTaskPayload(t *asynq.Task, pogo interface{}) error {
	err := msgpack.DecodeMsgPack(t.Payload(), &pogo)
	if err != nil {
		err = json.Unmarshal(t.Payload(), &pogo)
		if err != nil {
			return err
		}
	}
	return err
}
