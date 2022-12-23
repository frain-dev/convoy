package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/google/uuid"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrInvalidEventDeliveryStatus  = errors.New("only successful events can be force resent")
	ErrNoValidEndpointFound        = errors.New("no valid endpoint found")
	ErrNoValidOwnerIDEndpointFound = errors.New("owner ID has no configured endpoints")
	ErrInvalidEndpointID           = errors.New("please provide an endpoint ID")
)

type EventService struct {
	endpointRepo      datastore.EndpointRepository
	sourceRepo        datastore.SourceRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	queue             queue.Queuer
	subRepo           datastore.SubscriptionRepository
	cache             cache.Cache
	searcher          searcher.Searcher
	deviceRepo        datastore.DeviceRepository
}

type createEvent struct {
	Raw           string
	Data          json.RawMessage
	EventType     string
	EndpointID    string
	CustomHeaders map[string]string
}

func NewEventService(
	endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository,
	queue queue.Queuer, cache cache.Cache, seacher searcher.Searcher, subRepo datastore.SubscriptionRepository, sourceRepo datastore.SourceRepository, deviceRepo datastore.DeviceRepository,
) *EventService {
	return &EventService{endpointRepo: endpointRepo, eventRepo: eventRepo, eventDeliveryRepo: eventDeliveryRepo, queue: queue, cache: cache, searcher: seacher, subRepo: subRepo, sourceRepo: sourceRepo, deviceRepo: deviceRepo}
}

func (e *EventService) CreateEvent(ctx context.Context, newMessage *models.Event, g *datastore.Project) (*datastore.Event, error) {
	if g == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while creating event - invalid project"))
	}

	if err := util.Validate(newMessage); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if util.IsStringEmpty(newMessage.AppID) && util.IsStringEmpty(newMessage.EndpointID) {
		return nil, util.NewServiceError(http.StatusBadRequest, ErrInvalidEndpointID)
	}

	endpoints, err := e.FindEndpoints(ctx, newMessage)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if len(endpoints) == 0 {
		return nil, util.NewServiceError(http.StatusBadRequest, ErrNoValidEndpointFound)
	}

	createEvent := &createEvent{
		Data:          newMessage.Data,
		EventType:     newMessage.EventType,
		EndpointID:    newMessage.EndpointID,
		Raw:           string(newMessage.Data),
		CustomHeaders: newMessage.CustomHeaders,
	}

	event, err := e.createEvent(ctx, endpoints, createEvent, g)
	if err != nil {
		return nil, err
	}

	return event, nil
}

func (e *EventService) CreateFanoutEvent(ctx context.Context, newMessage *models.FanoutEvent, g *datastore.Project) (*datastore.Event, error) {
	if g == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while creating event - invalid project"))
	}

	if err := util.Validate(newMessage); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	endpoints, err := e.endpointRepo.FindEndpointsByOwnerID(ctx, g.UID, newMessage.OwnerID)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if len(endpoints) == 0 {
		return nil, util.NewServiceError(http.StatusBadRequest, ErrNoValidOwnerIDEndpointFound)
	}

	createEvent := &createEvent{
		Data:          newMessage.Data,
		EventType:     newMessage.EventType,
		Raw:           string(newMessage.Data),
		CustomHeaders: newMessage.CustomHeaders,
	}

	event, err := e.createEvent(ctx, endpoints, createEvent, g)
	if err != nil {
		return nil, err
	}

	return event, nil
}

func (e *EventService) ReplayEvent(ctx context.Context, event *datastore.Event, g *datastore.Project) error {
	taskName := convoy.CreateEventProcessor

	createEvent := task.CreateEvent{
		Event: *event,
	}

	eventByte, err := json.Marshal(createEvent)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, err)
	}
	payload := json.RawMessage(eventByte)

	job := &queue.Job{
		ID:      event.UID,
		Payload: payload,
		Delay:   0,
	}
	err = e.queue.Write(taskName, convoy.CreateEventQueue, job)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("replay_event: failed to write event to the queue")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to write event to queue"))
	}

	return nil
}

func (e *EventService) BatchReplayEvents(ctx context.Context, filter *datastore.Filter) (int, int, error) {
	events, _, err := e.eventRepo.LoadEventsPaged(ctx, filter)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch events")
		return 0, 0, util.NewServiceError(http.StatusInternalServerError, errors.New("failed to fetch event deliveries"))
	}

	failures := 0
	for _, ev := range events {
		err := e.ReplayEvent(ctx, &ev, filter.Project)
		if err != nil {
			failures++
			log.FromContext(ctx).WithError(err).Error("an item in the batch replay failed")
		}
	}

	successes := len(events) - failures
	return successes, failures, nil
}

func (e *EventService) GetEvent(ctx context.Context, id string) (*datastore.Event, error) {
	event, err := e.eventRepo.FindEventByID(ctx, id)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find event by id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find event by id"))
	}

	return event, nil
}

func (e *EventService) Search(ctx context.Context, filter *datastore.Filter) ([]datastore.Event, datastore.PaginationData, error) {
	var events []datastore.Event
	ids, paginationData, err := e.searcher.Search(filter.Project.UID, &datastore.SearchFilter{
		Query: filter.Query,
		FilterBy: datastore.FilterBy{
			EndpointID:   filter.EndpointID,
			SourceID:     filter.SourceID,
			ProjectID:    filter.Project.UID,
			SearchParams: filter.SearchParams,
		},
		Pageable: filter.Pageable,
	})
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch events from search backend")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusBadRequest, err)
	}

	events, err = e.eventRepo.FindEventsByIDs(ctx, ids)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch events from event ids")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusBadRequest, err)
	}

	return events, paginationData, err
}

func (e *EventService) GetEventDelivery(ctx context.Context, id string) (*datastore.EventDelivery, error) {
	eventDelivery, err := e.eventDeliveryRepo.FindEventDeliveryByID(ctx, id)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find event delivery by id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find event delivery by id"))
	}

	return eventDelivery, nil
}

func (e *EventService) BatchRetryEventDelivery(ctx context.Context, filter *datastore.Filter) (int, int, error) {
	deliveries, _, err := e.eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, filter.Project.UID, filter.EndpointIDs, filter.EventID, filter.Status, filter.SearchParams, filter.Pageable)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch event deliveries by ids")
		return 0, 0, util.NewServiceError(http.StatusInternalServerError, errors.New("failed to fetch event deliveries"))
	}

	failures := 0
	for _, delivery := range deliveries {
		err := e.RetryEventDelivery(ctx, &delivery, filter.Project)
		if err != nil {
			failures++
			log.FromContext(ctx).WithError(err).Error("an item in the batch retry failed")
		}
	}

	successes := len(deliveries) - failures
	return successes, failures, nil
}

func (e *EventService) CountAffectedEvents(ctx context.Context, filter *datastore.Filter) (int64, error) {
	count, err := e.eventRepo.CountEvents(ctx, filter)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("an error occurred while fetching event")
		return 0, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching event deliveries"))
	}

	return count, nil
}

func (e *EventService) CountAffectedEventDeliveries(ctx context.Context, filter *datastore.Filter) (int64, error) {
	count, err := e.eventDeliveryRepo.CountEventDeliveries(ctx, filter.Project.UID, filter.EndpointIDs, filter.EventID, filter.Status, filter.SearchParams)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("an error occurred while fetching event deliveries")
		return 0, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching event deliveries"))
	}

	return count, nil
}

func (e *EventService) ForceResendEventDeliveries(ctx context.Context, ids []string, g *datastore.Project) (int, int, error) {
	deliveries, err := e.eventDeliveryRepo.FindEventDeliveriesByIDs(ctx, ids)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch event deliveries by ids")
		return 0, 0, util.NewServiceError(http.StatusInternalServerError, errors.New("failed to fetch event deliveries"))
	}

	err = e.validateEventDeliveryStatus(deliveries)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("event delivery status validation failed")
		return 0, 0, util.NewServiceError(http.StatusBadRequest, err)
	}

	failures := 0
	for _, delivery := range deliveries {
		err := e.forceResendEventDelivery(ctx, &delivery, g)
		if err != nil {
			failures++
			log.FromContext(ctx).WithError(err).Error("an item in the force resend batch failed")
		}
	}

	successes := len(deliveries) - failures
	return successes, failures, nil
}

func (e *EventService) GetEventsPaged(ctx context.Context, filter *datastore.Filter) ([]datastore.Event, datastore.PaginationData, error) {
	events, paginationData, err := e.eventRepo.LoadEventsPaged(ctx, filter)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch events")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching events"))
	}

	return events, paginationData, nil
}

func (e *EventService) GetEventDeliveriesPaged(ctx context.Context, filter *datastore.Filter) ([]datastore.EventDelivery, datastore.PaginationData, error) {
	deliveries, paginationData, err := e.eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, filter.Project.UID, filter.EndpointIDs, filter.EventID, filter.Status, filter.SearchParams, filter.Pageable)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch event deliveries")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching event deliveries"))
	}

	return deliveries, paginationData, nil
}

func (e *EventService) ResendEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Project) error {
	err := e.RetryEventDelivery(ctx, eventDelivery, g)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to resend event delivery")
		return util.NewServiceError(http.StatusBadRequest, err)
	}

	return nil
}

func (e *EventService) RetryEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Project) error {
	switch eventDelivery.Status {
	case datastore.SuccessEventStatus:
		return errors.New("event already sent")
	case datastore.ScheduledEventStatus,
		datastore.ProcessingEventStatus,
		datastore.RetryEventStatus:
		return errors.New("cannot resend event that did not fail previously")
	}

	endpoint, err := e.endpointRepo.FindEndpointByID(ctx, eventDelivery.EndpointID)
	if err != nil {
		return datastore.ErrEndpointNotFound
	}

	if endpoint.Status == datastore.PendingEndpointStatus {
		return errors.New("endpoint is being re-activated")
	}

	if endpoint.Status == datastore.InactiveEndpointStatus {
		err = e.endpointRepo.UpdateEndpointStatus(context.Background(), eventDelivery.ProjectID, eventDelivery.EndpointID, datastore.PendingEndpointStatus)
		if err != nil {
			return errors.New("failed to update endpoint status")
		}
	}

	return e.requeueEventDelivery(ctx, eventDelivery, g)
}

func (e *EventService) forceResendEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Project) error {
	endpoint, err := e.endpointRepo.FindEndpointByID(ctx, eventDelivery.EndpointID)
	if err != nil {
		return datastore.ErrEndpointNotFound
	}

	if endpoint.Status != datastore.ActiveEndpointStatus {
		return errors.New("force resend to an inactive or pending endpoint is not allowed")
	}

	return e.requeueEventDelivery(ctx, eventDelivery, g)
}

func (e *EventService) validateEventDeliveryStatus(deliveries []datastore.EventDelivery) error {
	for _, delivery := range deliveries {
		if delivery.Status != datastore.SuccessEventStatus {
			return ErrInvalidEventDeliveryStatus
		}
	}

	return nil
}

func (e *EventService) requeueEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Project) error {
	eventDelivery.Status = datastore.ScheduledEventStatus
	err := e.eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, *eventDelivery, datastore.ScheduledEventStatus)
	if err != nil {
		return errors.New("an error occurred while trying to resend event")
	}

	taskName := convoy.EventProcessor

	job := &queue.Job{
		ID:      eventDelivery.UID,
		Payload: json.RawMessage(eventDelivery.UID),
		Delay:   1 * time.Second,
	}
	err = e.queue.Write(taskName, convoy.EventQueue, job)
	if err != nil {
		return fmt.Errorf("error occurred re-enqueing old event - %s: %v", eventDelivery.UID, err)
	}
	return nil
}

func (e *EventService) getCustomHeaders(customHeaders map[string]string) httpheader.HTTPHeader {
	var headers map[string][]string

	if customHeaders != nil {
		headers = make(map[string][]string)

		for key, value := range customHeaders {
			headers[key] = []string{value}
		}
	}

	return headers
}

func (e *EventService) createEvent(ctx context.Context, endpoints []datastore.Endpoint, newMessage *createEvent, g *datastore.Project) (*datastore.Event, error) {
	var endpointIDs []string

	for _, endpoint := range endpoints {
		endpointIDs = append(endpointIDs, endpoint.UID)
	}

	event := &datastore.Event{
		UID:       uuid.New().String(),
		EventType: datastore.EventType(newMessage.EventType),
		Data:      newMessage.Data,
		Raw:       newMessage.Raw,
		Headers:   e.getCustomHeaders(newMessage.CustomHeaders),
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
		Endpoints: endpointIDs,
		ProjectID: g.UID,
	}

	if (g.Config == nil || g.Config.Strategy == nil) ||
		(g.Config.Strategy != nil && g.Config.Strategy.Type != datastore.LinearStrategyProvider && g.Config.Strategy.Type != datastore.ExponentialStrategyProvider) {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("retry strategy not defined in configuration"))
	}

	taskName := convoy.CreateEventProcessor

	createEvent := task.CreateEvent{
		Event:              *event,
		CreateSubscription: !util.IsStringEmpty(newMessage.EndpointID),
	}

	eventByte, err := json.Marshal(createEvent)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	payload := json.RawMessage(eventByte)

	job := &queue.Job{
		ID:      event.UID,
		Payload: payload,
		Delay:   0,
	}
	err = e.queue.Write(taskName, convoy.CreateEventQueue, job)
	if err != nil {
		log.FromContext(ctx).Errorf("Error occurred sending new event to the queue %s", err)
	}

	return event, nil
}

func (e *EventService) FindEndpoints(ctx context.Context, newMessage *models.Event) ([]datastore.Endpoint, error) {
	var endpoints []datastore.Endpoint

	if !util.IsStringEmpty(newMessage.EndpointID) {
		endpoint, err := e.endpointRepo.FindEndpointByID(ctx, newMessage.EndpointID)
		if err != nil {
			return endpoints, err
		}

		endpoints = append(endpoints, *endpoint)
		return endpoints, nil
	}

	if !util.IsStringEmpty(newMessage.AppID) {
		endpoints, err := e.endpointRepo.FindEndpointsByAppID(ctx, newMessage.AppID)
		if err != nil {
			return endpoints, err
		}

		return endpoints, nil
	}

	return endpoints, nil
}
