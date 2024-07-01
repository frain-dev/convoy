package services

import (
	"context"
	"encoding/json"
	"errors"
	"gopkg.in/guregu/null.v4"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/worker/task"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
)

type CreateFanoutEventService struct {
	EndpointRepo   datastore.EndpointRepository
	EventRepo      datastore.EventRepository
	PortalLinkRepo datastore.PortalLinkRepository
	Queue          queue.Queuer

	NewMessage *models.FanoutEvent
	Project    *datastore.Project
}

var (
	ErrInvalidEventDeliveryStatus  = errors.New("only successful events can be force resent")
	ErrNoValidEndpointFound        = errors.New("no valid endpoint found")
	ErrNoValidOwnerIDEndpointFound = errors.New("owner ID has no configured endpoints")
	ErrInvalidEndpointID           = errors.New("please provide an endpoint ID")
)

type newEvent struct {
	UID            string
	Raw            string
	Data           json.RawMessage
	EventType      string
	EndpointID     string
	CustomHeaders  map[string]string
	IdempotencyKey string
	IsDuplicate    bool
	AcknowledgedAt time.Time
}

func (e *CreateFanoutEventService) Run(ctx context.Context) (*datastore.Event, error) {
	if e.Project == nil {
		return nil, &ServiceError{ErrMsg: "an error occurred while creating event - invalid project"}
	}

	if err := util.Validate(e.NewMessage); err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	var isDuplicate bool
	if !util.IsStringEmpty(e.NewMessage.IdempotencyKey) {
		events, err := e.EventRepo.FindEventsByIdempotencyKey(ctx, e.Project.UID, e.NewMessage.IdempotencyKey)
		if err != nil {
			return nil, &ServiceError{ErrMsg: err.Error()}
		}

		isDuplicate = len(events) > 0
	}

	endpoints, err := e.EndpointRepo.FindEndpointsByOwnerID(ctx, e.Project.UID, e.NewMessage.OwnerID)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if len(endpoints) == 0 {
		_, err := e.PortalLinkRepo.FindPortalLinkByOwnerID(ctx, e.Project.UID, e.NewMessage.OwnerID)
		if err != nil {
			if errors.Is(err, datastore.ErrPortalLinkNotFound) {
				return nil, &ServiceError{ErrMsg: ErrNoValidOwnerIDEndpointFound.Error()}
			}

			return nil, &ServiceError{ErrMsg: err.Error()}
		}
	}

	ev := &newEvent{
		UID:            ulid.Make().String(),
		Data:           e.NewMessage.Data,
		EventType:      e.NewMessage.EventType,
		IdempotencyKey: e.NewMessage.IdempotencyKey,
		Raw:            string(e.NewMessage.Data),
		CustomHeaders:  e.NewMessage.CustomHeaders,
		IsDuplicate:    isDuplicate,
		AcknowledgedAt: time.Now(),
	}

	event, err := createEvent(ctx, endpoints, ev, e.Project, e.Queue)
	if err != nil {
		return nil, err
	}

	return event, nil
}

func createEvent(ctx context.Context, endpoints []datastore.Endpoint, newMessage *newEvent, g *datastore.Project, queuer queue.Queuer) (*datastore.Event, error) {
	var endpointIDs []string

	for _, endpoint := range endpoints {
		endpointIDs = append(endpointIDs, endpoint.UID)
	}

	event := &datastore.Event{
		UID:              newMessage.UID,
		EventType:        datastore.EventType(newMessage.EventType),
		Data:             newMessage.Data,
		Raw:              newMessage.Raw,
		IdempotencyKey:   newMessage.IdempotencyKey,
		IsDuplicateEvent: newMessage.IsDuplicate,
		Headers:          getCustomHeaders(newMessage.CustomHeaders),
		Endpoints:        endpointIDs,
		ProjectID:        g.UID,
		AcknowledgedAt:   null.TimeFrom(time.Now()),
	}

	if (g.Config == nil || g.Config.Strategy == nil) ||
		(g.Config.Strategy != nil && g.Config.Strategy.Type != datastore.LinearStrategyProvider && g.Config.Strategy.Type != datastore.ExponentialStrategyProvider) {
		return nil, &ServiceError{ErrMsg: "retry strategy not defined in configuration"}
	}

	e := task.CreateEvent{
		Event:              event,
		CreateSubscription: !util.IsStringEmpty(newMessage.EndpointID),
	}

	eventByte, err := msgpack.EncodeMsgPack(e)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	job := &queue.Job{
		ID:      event.UID,
		Payload: eventByte,
		Delay:   0,
	}
	err = queuer.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
	if err != nil {
		log.FromContext(ctx).Errorf("Error occurred sending new event to the queue %s", err)
	}

	return event, nil
}

func getCustomHeaders(customHeaders map[string]string) httpheader.HTTPHeader {
	var headers map[string][]string

	if customHeaders != nil {
		headers = make(map[string][]string)

		for key, value := range customHeaders {
			headers[key] = []string{value}
		}
	}

	return headers
}
