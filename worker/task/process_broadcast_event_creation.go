package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrFailedToWriteToQueue = errors.New("failed to write to event delivery queue")
	defaultBroadcastDelay   = 30 * time.Second
)

type BroadcastEventChannel struct {
	SubscriptionsTable *memorystore.ITable
}

func NewBroadcastEventChannel(subTable memorystore.ITable) *BroadcastEventChannel {
	return &BroadcastEventChannel{
		SubscriptionsTable: &subTable,
	}
}

func (b *BroadcastEventChannel) GetConfig() *EventChannelConfig {
	return &EventChannelConfig{
		Channel:      "broadcast",
		DefaultDelay: defaultBroadcastDelay,
	}
}

func (b *BroadcastEventChannel) CreateEvent(ctx context.Context, t *asynq.Task, channel EventChannel, args EventChannelArgs) (*datastore.Event, error) {
	// Start a new trace span for event creation
	attributes := map[string]interface{}{
		"event.type": "broadcast.event.creation",
		"channel":    channel,
	}

	var broadcastEvent models.BroadcastEvent
	err := msgpack.DecodeMsgPack(t.Payload(), &broadcastEvent)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventBroadcastEventCreationError, attributes)
		return nil, &EndpointError{Err: fmt.Errorf("CODE: 1001, err: %s", err.Error()), delay: defaultBroadcastDelay}
	}

	attributes["project.id"] = broadcastEvent.ProjectID
	attributes["event.id"] = broadcastEvent.EventID

	project, err := args.projectRepo.FetchProjectByID(ctx, broadcastEvent.ProjectID)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventBroadcastEventCreationError, attributes)
		return nil, &EndpointError{Err: fmt.Errorf("CODE: 1002, err: %s", err.Error()), delay: defaultBroadcastDelay}
	}

	var isDuplicate bool
	if len(broadcastEvent.IdempotencyKey) > 0 {
		isDuplicate, err = args.eventRepo.FindEventsByIdempotencyKey(ctx, broadcastEvent.ProjectID, broadcastEvent.IdempotencyKey)
		if err != nil {
			tracer.AddEvent(ctx, tracer.EventBroadcastEventCreationError, attributes)
			return nil, &EndpointError{Err: fmt.Errorf("CODE: 1004, err: %s", err.Error()), delay: defaultBroadcastDelay}
		}
	}

	event := &datastore.Event{
		UID:              broadcastEvent.EventID,
		EventType:        datastore.EventType(broadcastEvent.EventType),
		ProjectID:        project.UID,
		SourceID:         broadcastEvent.SourceID,
		Data:             broadcastEvent.Data,
		IdempotencyKey:   broadcastEvent.IdempotencyKey,
		Headers:          getCustomHeaders(broadcastEvent.CustomHeaders),
		IsDuplicateEvent: isDuplicate,
		Raw:              "", // Skip Raw duplication - Data field is canonical (reduces payload size)
		Status:           datastore.PendingStatus,
		AcknowledgedAt:   null.TimeFrom(time.Now()),
	}

	err = updateEventMetadata(channel, event, false, args.logger)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventBroadcastEventCreationError, attributes)
		return nil, err
	}

	err = args.eventRepo.CreateEvent(ctx, event)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventBroadcastEventCreationError, attributes)
		return nil, &EndpointError{Err: fmt.Errorf("CODE: 1005, err: %s", err.Error()), delay: defaultBroadcastDelay}
	}

	tracer.AddEvent(ctx, tracer.EventBroadcastEventCreationSuccess, attributes)
	return event, nil
}

func (b *BroadcastEventChannel) MatchSubscriptions(ctx context.Context, metadata EventChannelMetadata, args EventChannelArgs) (*EventChannelSubResponse, error) {
	// Start a new trace span for subscription matching
	attributes := map[string]interface{}{
		"event.type": "broadcast.subscription.matching",
		"event.id":   metadata.Event.UID,
		"channel":    metadata.Config.Channel,
	}

	response := EventChannelSubResponse{}

	project, err := args.projectRepo.FetchProjectByID(ctx, metadata.Event.ProjectID)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventBroadcastSubscriptionMatchingErr, attributes)
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}

	attributes["project.id"] = project.UID

	broadcastEvent, err := args.eventRepo.FindEventByID(ctx, project.UID, metadata.Event.UID)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventBroadcastSubscriptionMatchingErr, attributes)
		return nil, &EndpointError{Err: err, delay: defaultDelay}
	}

	err = args.eventRepo.UpdateEventStatus(ctx, broadcastEvent, datastore.ProcessingStatus)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventBroadcastSubscriptionMatchingErr, attributes)
		return nil, err
	}

	subscriptionsTable := *b.SubscriptionsTable
	mKeys := memorystore.NewKey(project.UID, "*")
	matchAllSubs := getSubscriptionsFromRow(subscriptionsTable.Get(mKeys))

	key := memorystore.NewKey(project.UID, string(broadcastEvent.EventType))
	eventTypeSubs := getSubscriptionsFromRow(subscriptionsTable.Get(key))

	subscriptions := make([]datastore.Subscription, 0, len(matchAllSubs)+len(eventTypeSubs))
	subscriptions = append(subscriptions, eventTypeSubs...)
	subscriptions = append(subscriptions, matchAllSubs...)

	args.logger.DebugContext(ctx, "matching subscriptions using filter", "event.id", broadcastEvent.UID)

	subscriptions, err = matchSubscriptionsUsingFilter(ctx, broadcastEvent, args.subRepo, args.filterRepo, args.licenser, subscriptions, true, args.logger)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventBroadcastSubscriptionMatchingErr, attributes)
		return nil, &EndpointError{Err: fmt.Errorf("failed to match subscriptions using filter, err: %s", err.Error()), delay: defaultBroadcastDelay}
	}

	es, ss := getEndpointIDs(subscriptions)
	broadcastEvent.Endpoints = es

	err = args.eventRepo.UpdateEventEndpoints(ctx, broadcastEvent, es)
	if err != nil {
		tracer.AddEvent(ctx, tracer.EventBroadcastSubscriptionMatchingErr, attributes)
		return nil, &EndpointError{Err: fmt.Errorf("CODE: 1011, err: %s", err.Error()), delay: defaultBroadcastDelay}
	}
	response.Event = broadcastEvent
	response.Project = project
	response.Subscriptions = ss
	response.IsDuplicateEvent = broadcastEvent.IsDuplicateEvent

	tracer.AddEvent(ctx, tracer.EventBroadcastSubscriptionMatchingOK, attributes)
	return &response, nil
}

func ProcessBroadcastEventCreation(
	ch *BroadcastEventChannel,
	deps EventProcessorDeps,
) func(context.Context, *asynq.Task) error {
	return ProcessEventCreationByChannel(
		ch,
		deps.EndpointRepo,
		deps.EventRepo,
		deps.ProjectRepo,
		deps.EventQueue,
		deps.SubRepo,
		deps.FilterRepo,
		deps.Licenser,
		deps.OAuth2TokenService,
		deps.Logger,
	)
}

func getEndpointIDs(subs []datastore.Subscription) ([]string, []datastore.Subscription) {
	subMap := make(map[string]*datastore.Subscription)
	endpointIds := make([]string, 0, len(subs))

	var sub *datastore.Subscription
	for i := range subs {
		sub = &subs[i]
		if sub.Type == datastore.SubscriptionTypeAPI && !util.IsStringEmpty(sub.EndpointID) {
			if _, ok := subMap[sub.EndpointID]; !ok {
				subMap[sub.EndpointID] = sub
				endpointIds = append(endpointIds, sub.EndpointID)
			}
		}
	}

	subscriptionsIds := make([]datastore.Subscription, 0, len(subMap))
	for _, s := range subMap {
		subscriptionsIds = append(subscriptionsIds, *s)
	}

	return endpointIds, subscriptionsIds
}

func getSubscriptionsFromRow(row *memorystore.Row) []datastore.Subscription {
	if row == nil {
		return []datastore.Subscription{}
	}

	subs, ok := row.Value().([]datastore.Subscription)
	if !ok {
		return []datastore.Subscription{}
	}

	return subs
}
