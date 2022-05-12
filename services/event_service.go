package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EventService struct {
	appRepo           datastore.ApplicationRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	eventQueue        queue.Queuer
	createEventQueue  queue.Queuer
	cache             cache.Cache
}

func NewEventService(appRepo datastore.ApplicationRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository,
	eventQueue queue.Queuer, createEventQueue queue.Queuer, cache cache.Cache) *EventService {
	return &EventService{appRepo: appRepo, eventRepo: eventRepo, eventDeliveryRepo: eventDeliveryRepo, eventQueue: eventQueue, createEventQueue: createEventQueue, cache: cache}
}

func (e *EventService) CreateAppEvent(ctx context.Context, newMessage *models.Event, g *datastore.Group) (*datastore.Event, error) {
	if g == nil {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("an error occurred while creating event - invalid group"))
	}

	if err := util.Validate(newMessage); err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
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
			return nil, NewServiceError(statusCode, errors.New(msg))
		}

		err = e.cache.Set(ctx, appCacheKey, &app, time.Minute*5)
		if err != nil {
			return nil, err
		}
	}

	if len(app.Endpoints) == 0 {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("app has no configured endpoints"))
	}

	event := &datastore.Event{
		UID:       uuid.New().String(),
		EventType: datastore.EventType(newMessage.EventType),
		Data:      newMessage.Data,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
		AppMetadata: &datastore.AppMetadata{
			Title:        app.Title,
			UID:          app.UID,
			GroupID:      app.GroupID,
			SupportEmail: app.SupportEmail,
		},
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	if g.Config.Strategy.Type != config.DefaultStrategyProvider && g.Config.Strategy.Type != config.ExponentialBackoffStrategyProvider {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("retry strategy not defined in configuration"))
	}

	taskName := convoy.CreateEventProcessor.SetPrefix(g.Name)
	err = e.createEventQueue.WriteEvent(context.Background(), taskName, event, 1*time.Second)
	if err != nil {
		log.Errorf("Error occurred sending new event to the queue %s", err)
	}

	return event, nil
}

func (e *EventService) GetAppEvent(ctx context.Context, id string) (*datastore.Event, error) {
	event, err := e.eventRepo.FindEventByID(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to find event by id")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to find event by id"))
	}

	return event, nil
}

func (e *EventService) GetEventDelivery(ctx context.Context, id string) (*datastore.EventDelivery, error) {
	eventDelivery, err := e.eventDeliveryRepo.FindEventDeliveryByID(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to find event delivery by id")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to find event delivery by id"))
	}

	return eventDelivery, nil
}

func (e *EventService) BatchRetryEventDelivery(ctx context.Context, filter *datastore.Filter) (int, int, error) {
	deliveries, _, err := e.eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, filter.Group.UID, filter.AppID, filter.EventID, filter.Status, filter.SearchParams, filter.Pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch event deliveries by ids")
		return 0, 0, NewServiceError(http.StatusInternalServerError, errors.New("failed to fetch event deliveries"))
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
		return 0, NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching event deliveries"))
	}

	return count, nil
}

func (e *EventService) ForceResendEventDeliveries(ctx context.Context, ids []string, g *datastore.Group) (int, int, error) {
	var deliveries []datastore.EventDelivery
	deliveries, err := e.eventDeliveryRepo.FindEventDeliveriesByIDs(ctx, ids)
	if err != nil {
		log.WithError(err).Error("failed to fetch event deliveries by ids")
		return 0, 0, NewServiceError(http.StatusInternalServerError, errors.New("failed to fetch event deliveries"))
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
	m, paginationData, err := e.eventRepo.LoadEventsPaged(ctx, filter.Group.UID, filter.AppID, filter.SearchParams, filter.Pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch events")
		return nil, datastore.PaginationData{}, NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching events"))
	}

	return m, paginationData, nil
}

func (e *EventService) GetEventDeliveriesPaged(ctx context.Context, filter *datastore.Filter) ([]datastore.EventDelivery, datastore.PaginationData, error) {
	ed, paginationData, err := e.eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, filter.Group.UID, filter.AppID, filter.EventID, filter.Status, filter.SearchParams, filter.Pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch event deliveries")
		return nil, datastore.PaginationData{}, NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching event deliveries"))
	}

	return ed, paginationData, nil
}

func (e *EventService) ResendEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Group) error {
	err := e.RetryEventDelivery(ctx, eventDelivery, g)
	if err != nil {
		log.WithError(err).Error("failed to resend event delivery")
		return NewServiceError(http.StatusBadRequest, err)
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

	em := eventDelivery.EndpointMetadata
	endpoint, err := e.appRepo.FindApplicationEndpointByID(context.Background(), eventDelivery.AppMetadata.UID, em.UID)
	if err != nil {
		log.WithError(err).Error("failed to find endpoint")
		return errors.New("cannot find endpoint")
	}

	if endpoint.Status == datastore.PendingEndpointStatus {
		return errors.New("endpoint is being re-activated")
	}

	if endpoint.Status == datastore.InactiveEndpointStatus {
		pendingEndpoints := []string{em.UID}

		err = e.appRepo.UpdateApplicationEndpointsStatus(context.Background(), eventDelivery.AppMetadata.UID, pendingEndpoints, datastore.PendingEndpointStatus)
		if err != nil {
			return errors.New("failed to update endpoint status")
		}
	}

	return e.requeueEventDelivery(ctx, eventDelivery, g)
}

func (e *EventService) forceResendEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Group) error {
	if eventDelivery.Status != datastore.SuccessEventStatus {
		return errors.New("only successful events can be force resent")
	}

	em := eventDelivery.EndpointMetadata
	endpoint, err := e.appRepo.FindApplicationEndpointByID(context.Background(), eventDelivery.AppMetadata.UID, em.UID)
	if err != nil {
		return errors.New("cannot find endpoint")
	}

	if endpoint.Status != datastore.ActiveEndpointStatus {
		return errors.New("force resend to an inactive or pending endpoint is not allowed")
	}

	return e.requeueEventDelivery(ctx, eventDelivery, g)
}

func (e *EventService) requeueEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Group) error {
	eventDelivery.Status = datastore.ScheduledEventStatus
	err := e.eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, *eventDelivery, datastore.ScheduledEventStatus)
	if err != nil {
		return errors.New("an error occurred while trying to resend event")
	}

	taskName := convoy.EventProcessor.SetPrefix(g.Name)
	err = e.eventQueue.WriteEventDelivery(context.Background(), taskName, eventDelivery, 1*time.Second)
	if err != nil {
		return fmt.Errorf("error occurred re-enqueing old event - %s: %v", eventDelivery.UID, err)
	}
	return nil
}
