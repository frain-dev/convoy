package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
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
}

func (e *EventService) CreateAppEvent(ctx context.Context, newMessage *models.Event, g *datastore.Group) (*datastore.Event, error) {
	eventType := newMessage.EventType
	d := newMessage.Data

	if err := util.Validate(newMessage); err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	app, err := e.appRepo.FindApplicationByID(ctx, newMessage.AppID)
	if err != nil {

		msg := "an error occurred while retrieving app details"
		statusCode := http.StatusInternalServerError

		if errors.Is(err, datastore.ErrApplicationNotFound) {
			msg = err.Error()
			statusCode = http.StatusNotFound
		}

		log.Debugln("error while fetching app - ", err)

		return nil, NewServiceError(statusCode, errors.New(msg))
	}

	if len(app.Endpoints) == 0 {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("app has no configured endpoints"))
	}

	if app.IsDisabled {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("app is disabled, no events were sent"))
	}

	matchedEndpoints := matchEndpointsForDelivery(eventType, app.Endpoints, nil)

	event := &datastore.Event{
		UID:              uuid.New().String(),
		EventType:        datastore.EventType(eventType),
		MatchedEndpoints: len(matchedEndpoints),
		Data:             d,
		CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
		AppMetadata: &datastore.AppMetadata{
			Title:        app.Title,
			UID:          app.UID,
			GroupID:      app.GroupID,
			SupportEmail: app.SupportEmail,
		},
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err = e.eventRepo.CreateEvent(ctx, event)
	if err != nil {
		return nil, NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while creating event"))
	}

	var intervalSeconds uint64
	var retryLimit uint64
	if string(g.Config.Strategy.Type) == string(config.DefaultStrategyProvider) {
		intervalSeconds = g.Config.Strategy.Default.IntervalSeconds
		retryLimit = g.Config.Strategy.Default.RetryLimit
	} else if string(g.Config.Strategy.Type) == string(config.ExponentialBackoffStrategyProvider) {
		intervalSeconds = 0
		retryLimit = g.Config.Strategy.ExponentialBackoff.RetryLimit
	} else {
		return nil, NewServiceError(http.StatusInternalServerError, errors.New("retry strategy not defined in configuration"))
	}

	for _, v := range matchedEndpoints {
		eventDelivery := &datastore.EventDelivery{
			UID: uuid.New().String(),
			EventMetadata: &datastore.EventMetadata{
				UID:       event.UID,
				EventType: event.EventType,
			},
			EndpointMetadata: &datastore.EndpointMetadata{
				UID:       v.UID,
				TargetURL: v.TargetURL,
				Status:    v.Status,
				Secret:    v.Secret,
				Sent:      false,
			},
			AppMetadata: &datastore.AppMetadata{
				UID:          app.UID,
				Title:        app.Title,
				GroupID:      app.GroupID,
				SupportEmail: app.SupportEmail,
			},
			Metadata: &datastore.Metadata{
				Data:            event.Data,
				Strategy:        g.Config.Strategy.Type,
				NumTrials:       0,
				IntervalSeconds: intervalSeconds,
				RetryLimit:      retryLimit,
				NextSendTime:    primitive.NewDateTimeFromTime(time.Now()),
			},
			Status:           getEventDeliveryStatus(v),
			DeliveryAttempts: make([]datastore.DeliveryAttempt, 0),
			DocumentStatus:   datastore.ActiveDocumentStatus,
			CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
		}
		err = e.eventDeliveryRepo.CreateEventDelivery(ctx, eventDelivery)
		if err != nil {
			log.WithError(err).Error("error occurred creating event delivery")
		}

		taskName := convoy.EventProcessor.SetPrefix(g.Name)

		if eventDelivery.Status != datastore.DiscardedEventStatus {
			err = e.eventQueue.Write(ctx, taskName, eventDelivery, 1*time.Second)
			if err != nil {
				log.Errorf("Error occurred sending new event to the queue %s", err)
			}
		}

	}

	return event, nil
}

func getEventDeliveryStatus(endpoint datastore.Endpoint) datastore.EventDeliveryStatus {
	if endpoint.Status != datastore.ActiveEndpointStatus {
		return datastore.DiscardedEventStatus
	}

	return datastore.ScheduledEventStatus
}

func matchEndpointsForDelivery(ev string, endpoints, matched []datastore.Endpoint) []datastore.Endpoint {
	if len(endpoints) == 0 {
		return matched
	}

	if matched == nil {
		matched = make([]datastore.Endpoint, 0)
	}

	e := endpoints[0]
	for _, v := range e.Events {
		if v == ev || v == "*" {
			matched = append(matched, e)
			break
		}
	}

	return matchEndpointsForDelivery(ev, endpoints[1:], matched)
}
