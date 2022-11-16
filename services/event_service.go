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
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrInvalidEventDeliveryStatus = errors.New("only successful events can be force resent")

type EventService struct {
	appRepo           datastore.ApplicationRepository
	sourceRepo        datastore.SourceRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	queue             queue.Queuer
	subRepo           datastore.SubscriptionRepository
	cache             cache.Cache
	searcher          searcher.Searcher
	deviceRepo        datastore.DeviceRepository
}

func NewEventService(
	appRepo datastore.ApplicationRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository,
	queue queue.Queuer, cache cache.Cache, seacher searcher.Searcher, subRepo datastore.SubscriptionRepository, sourceRepo datastore.SourceRepository, deviceRepo datastore.DeviceRepository,
) *EventService {
	return &EventService{appRepo: appRepo, eventRepo: eventRepo, eventDeliveryRepo: eventDeliveryRepo, queue: queue, cache: cache, searcher: seacher, subRepo: subRepo, sourceRepo: sourceRepo, deviceRepo: deviceRepo}
}

func (e *EventService) CreateAppEvent(ctx context.Context, newMessage *models.Event, g *datastore.Group) (*datastore.Event, error) {
	if g == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while creating event - invalid group"))
	}

	if err := util.Validate(newMessage); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	var app *datastore.Application
	appCacheKey := convoy.ApplicationsCacheKey.Get(newMessage.AppID).String()

	err := e.cache.Get(ctx, appCacheKey, &app)
	if err != nil {
		return nil, err
	}

	if app == nil {
		app, err = e.appRepo.FindApplicationByID(ctx, newMessage.AppID)
		if err != nil {

			msg := "an error occurred while retrieving app details"
			statusCode := http.StatusBadRequest

			if errors.Is(err, datastore.ErrApplicationNotFound) {
				msg = err.Error()
				statusCode = http.StatusNotFound
			}

			log.WithError(err).Error("failed to fetch app")
			return nil, util.NewServiceError(statusCode, errors.New(msg))
		}

		err = e.cache.Set(ctx, appCacheKey, &app, time.Minute*5)
		if err != nil {
			return nil, err
		}
	}

	if len(app.Endpoints) == 0 {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("app has no configured endpoints"))
	}

	// TODO(daniel): consider adding this check
	// if app.GroupID != g.UID {
	//	return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("unauthorized to access app"))
	// }

	event := &datastore.Event{
		UID:            uuid.New().String(),
		EventType:      datastore.EventType(newMessage.EventType),
		Data:           newMessage.Data,
		Headers:        e.getCustomHeaders(newMessage),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		AppID:          app.UID,
		GroupID:        app.GroupID,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	if (g.Config == nil || g.Config.Strategy == nil) ||
		(g.Config.Strategy != nil && g.Config.Strategy.Type != datastore.LinearStrategyProvider && g.Config.Strategy.Type != datastore.ExponentialStrategyProvider) {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("retry strategy not defined in configuration"))
	}

	taskName := convoy.CreateEventProcessor
	eventByte, err := json.Marshal(event)
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
		log.Errorf("Error occurred sending new event to the queue %s", err)
	}

	return event, nil
}

func (e *EventService) ReplayAppEvent(ctx context.Context, event *datastore.Event, g *datastore.Group) error {
	taskName := convoy.CreateEventProcessor
	eventByte, err := json.Marshal(event)
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
		log.WithError(err).Error("replay_event: failed to write event to the queue")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to write event to queue"))
	}

	return nil
}

func (e *EventService) GetAppEvent(ctx context.Context, id string) (*datastore.Event, error) {
	event, err := e.eventRepo.FindEventByID(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to find event by id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find event by id"))
	}

	return event, nil
}

func (e *EventService) Search(ctx context.Context, filter *datastore.Filter) ([]datastore.Event, datastore.PaginationData, error) {
	var events []datastore.Event
	ids, paginationData, err := e.searcher.Search(filter.Group.UID, &datastore.SearchFilter{
		Query: filter.Query,
		FilterBy: datastore.FilterBy{
			AppID:        filter.AppID,
			SourceID:     filter.SourceID,
			GroupID:      filter.Group.UID,
			SearchParams: filter.SearchParams,
		},
		Pageable: filter.Pageable,
	})
	if err != nil {
		log.WithError(err).Error("failed to fetch events from search backend")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusBadRequest, err)
	}

	events, err = e.eventRepo.FindEventsByIDs(ctx, ids)
	if err != nil {
		log.WithError(err).Error("failed to fetch events from event ids")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusBadRequest, err)
	}

	return events, paginationData, err
}

func (e *EventService) GetEventDelivery(ctx context.Context, id string) (*datastore.EventDelivery, error) {
	eventDelivery, err := e.eventDeliveryRepo.FindEventDeliveryByID(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to find event delivery by id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find event delivery by id"))
	}

	return eventDelivery, nil
}

func (e *EventService) BatchRetryEventDelivery(ctx context.Context, filter *datastore.Filter) (int, int, error) {
	deliveries, _, err := e.eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, filter.Group.UID, filter.AppID, filter.EventID, filter.Status, filter.SearchParams, filter.Pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch event deliveries by ids")
		return 0, 0, util.NewServiceError(http.StatusInternalServerError, errors.New("failed to fetch event deliveries"))
	}

	failures := 0
	for _, delivery := range deliveries {
		err := e.RetryEventDelivery(ctx, &delivery, filter.Group)
		if err != nil {
			failures++
			log.WithError(err).Error("an item in the batch retry failed")
		}
	}

	successes := len(deliveries) - failures
	return successes, failures, nil
}

func (e *EventService) CountAffectedEventDeliveries(ctx context.Context, filter *datastore.Filter) (int64, error) {
	count, err := e.eventDeliveryRepo.CountEventDeliveries(ctx, filter.Group.UID, filter.AppID, filter.EventID, filter.Status, filter.SearchParams)
	if err != nil {
		log.WithError(err).Error("an error occurred while fetching event deliveries")
		return 0, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching event deliveries"))
	}

	return count, nil
}

func (e *EventService) ForceResendEventDeliveries(ctx context.Context, ids []string, g *datastore.Group) (int, int, error) {
	deliveries, err := e.eventDeliveryRepo.FindEventDeliveriesByIDs(ctx, ids)
	if err != nil {
		log.WithError(err).Error("failed to fetch event deliveries by ids")
		return 0, 0, util.NewServiceError(http.StatusInternalServerError, errors.New("failed to fetch event deliveries"))
	}

	err = e.validateEventDeliveryStatus(deliveries)
	if err != nil {
		log.WithError(err).Error("event delivery status validation failed")
		return 0, 0, util.NewServiceError(http.StatusBadRequest, err)
	}

	failures := 0
	for _, delivery := range deliveries {
		err := e.forceResendEventDelivery(ctx, &delivery, g)
		if err != nil {
			failures++
			log.WithError(err).Error("an item in the force resend batch failed")
		}
	}

	successes := len(deliveries) - failures
	return successes, failures, nil
}

func (e *EventService) GetEventsPaged(ctx context.Context, filter *datastore.Filter) ([]datastore.Event, datastore.PaginationData, error) {
	events, paginationData, err := e.eventRepo.LoadEventsPaged(ctx, filter)
	if err != nil {
		log.WithError(err).Error("failed to fetch events")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching events"))
	}

	return events, paginationData, nil
}

func (e *EventService) GetEventDeliveriesPaged(ctx context.Context, filter *datastore.Filter) ([]datastore.EventDelivery, datastore.PaginationData, error) {
	deliveries, paginationData, err := e.eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, filter.Group.UID, filter.AppID, filter.EventID, filter.Status, filter.SearchParams, filter.Pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch event deliveries")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching event deliveries"))
	}

	return deliveries, paginationData, nil
}

func (e *EventService) ResendEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Group) error {
	err := e.RetryEventDelivery(ctx, eventDelivery, g)
	if err != nil {
		log.WithError(err).Error("failed to resend event delivery")
		return util.NewServiceError(http.StatusBadRequest, err)
	}

	return nil
}

func (e *EventService) RetryEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Group) error {
	switch eventDelivery.Status {
	case datastore.SuccessEventStatus:
		return errors.New("event already sent")
	case datastore.ScheduledEventStatus,
		datastore.ProcessingEventStatus,
		datastore.RetryEventStatus:
		return errors.New("cannot resend event that did not fail previously")
	}

	sub, err := e.subRepo.FindSubscriptionByID(ctx, g.UID, eventDelivery.SubscriptionID)
	if err != nil {
		return ErrSubscriptionNotFound
	}

	if sub.Status == datastore.PendingSubscriptionStatus {
		return errors.New("subscription is being re-activated")
	}

	if sub.Status == datastore.InactiveSubscriptionStatus {
		err = e.subRepo.UpdateSubscriptionStatus(context.Background(), eventDelivery.GroupID, eventDelivery.SubscriptionID, datastore.PendingSubscriptionStatus)
		if err != nil {
			return errors.New("failed to update subscription status")
		}
	}

	return e.requeueEventDelivery(ctx, eventDelivery, g)
}

func (e *EventService) forceResendEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Group) error {
	sub, err := e.subRepo.FindSubscriptionByID(ctx, g.UID, eventDelivery.SubscriptionID)
	if err != nil {
		return ErrSubscriptionNotFound
	}

	if sub.Status != datastore.ActiveSubscriptionStatus {
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

func (e *EventService) requeueEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Group) error {
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

func (e *EventService) getCustomHeaders(event *models.Event) httpheader.HTTPHeader {
	var headers map[string][]string

	if event.CustomHeaders != nil {
		headers = make(map[string][]string)

		for key, value := range event.CustomHeaders {
			headers[key] = []string{value}
		}
	}

	return headers
}
