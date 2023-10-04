package services

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/oklog/ulid/v2"
)

var (
	ErrInvalidEventDeliveryStatus  = errors.New("only successful events can be force resent")
	ErrNoValidEndpointFound        = errors.New("no valid endpoint found")
	ErrNoValidOwnerIDEndpointFound = errors.New("owner ID has no configured endpoints")
	ErrInvalidEndpointID           = errors.New("please provide an endpoint ID")
)

type CreateEventService struct {
	EndpointRepo datastore.EndpointRepository
	EventRepo    datastore.EventRepository
	Queue        queue.Queuer

	NewMessage *models.CreateEvent
	Project    *datastore.Project
}

type newEvent struct {
	Raw            string
	Data           json.RawMessage
	EventType      string
	EndpointID     string
	CustomHeaders  map[string]string
	IdempotencyKey string
	IsDuplicate    bool
}

func (c *CreateEventService) Run(ctx context.Context) (*datastore.Event, error) {
	var isDuplicate bool
	if !util.IsStringEmpty(c.NewMessage.IdempotencyKey) {
		events, err := c.EventRepo.FindEventsByIdempotencyKey(ctx, c.Project.UID, c.NewMessage.IdempotencyKey)
		if err != nil {
			return nil, &ServiceError{ErrMsg: err.Error()}
		}

		isDuplicate = len(events) > 0
	}

	if c.Project == nil {
		return nil, &ServiceError{ErrMsg: "an error occurred while creating event - invalid project"}
	}

	if util.IsStringEmpty(c.NewMessage.AppID) && util.IsStringEmpty(c.NewMessage.EndpointID) {
		return nil, &ServiceError{ErrMsg: ErrInvalidEndpointID.Error()}
	}

	endpoints, err := c.findEndpoints(ctx, c.NewMessage, c.Project)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find endpoints")
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if len(endpoints) == 0 {
		return nil, &ServiceError{ErrMsg: ErrNoValidEndpointFound.Error()}
	}

	newEvent := &newEvent{
		Data:           c.NewMessage.Data,
		EventType:      c.NewMessage.EventType,
		EndpointID:     c.NewMessage.EndpointID,
		Raw:            string(c.NewMessage.Data),
		CustomHeaders:  c.NewMessage.CustomHeaders,
		IdempotencyKey: c.NewMessage.IdempotencyKey,
		IsDuplicate:    isDuplicate,
	}

	event, err := createEvent(ctx, endpoints, newEvent, c.Project, c.Queue)
	if err != nil {
		return nil, err
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

func createEvent(ctx context.Context, endpoints []datastore.Endpoint, newMessage *newEvent, g *datastore.Project, queuer queue.Queuer) (*datastore.Event, error) {
	var endpointIDs []string

	for _, endpoint := range endpoints {
		endpointIDs = append(endpointIDs, endpoint.UID)
	}

	event := &datastore.Event{
		UID:              ulid.Make().String(),
		EventType:        datastore.EventType(newMessage.EventType),
		Data:             newMessage.Data,
		Raw:              newMessage.Raw,
		IdempotencyKey:   newMessage.IdempotencyKey,
		IsDuplicateEvent: newMessage.IsDuplicate,
		Headers:          getCustomHeaders(newMessage.CustomHeaders),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Endpoints:        endpointIDs,
		ProjectID:        g.UID,
	}

	if (g.Config == nil || g.Config.Strategy == nil) ||
		(g.Config.Strategy != nil && g.Config.Strategy.Type != datastore.LinearStrategyProvider && g.Config.Strategy.Type != datastore.ExponentialStrategyProvider) {
		return nil, &ServiceError{ErrMsg: "retry strategy not defined in configuration"}
	}

	e := task.CreateEvent{
		Event:              *event,
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

func (c *CreateEventService) findEndpoints(ctx context.Context, newMessage *models.CreateEvent, project *datastore.Project) ([]datastore.Endpoint, error) {
	var endpoints []datastore.Endpoint

	if !util.IsStringEmpty(newMessage.EndpointID) {
		endpoint, err := c.EndpointRepo.FindEndpointByID(ctx, newMessage.EndpointID, project.UID)
		if err != nil {
			return endpoints, err
		}

		endpoints = append(endpoints, *endpoint)
		return endpoints, nil
	}

	if !util.IsStringEmpty(newMessage.AppID) {
		endpoints, err := c.EndpointRepo.FindEndpointsByAppID(ctx, newMessage.AppID, project.UID)
		if err != nil {
			return endpoints, err
		}

		return endpoints, nil
	}

	return endpoints, nil
}
