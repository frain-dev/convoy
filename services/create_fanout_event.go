package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"go.opentelemetry.io/otel"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/httpheader"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
)

type CreateFanoutEventService struct {
	EndpointRepo   datastore.EndpointRepository
	EventRepo      datastore.EventRepository
	PortalLinkRepo datastore.PortalLinkRepository
	Queue          queue.Queuer

	NewMessage *models.FanoutEvent
	Project    *datastore.Project
	Logger     log.Logger
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

func (e *CreateFanoutEventService) Run(ctx context.Context) (event *datastore.Event, err error) {
	ctx, span := otel.Tracer(tracer.TracerNameServices).Start(ctx, tracer.SpanServicesEventCreateFanout)
	defer func() {
		tracer.RecordError(span, err)
		span.End()
	}()

	serviceStart := time.Now()

	if e.Project == nil {
		return nil, &ServiceError{ErrMsg: "an error occurred while creating event - invalid project"}
	}
	span.SetAttributes(tracer.AttrProjectID.String(e.Project.UID))
	if e.NewMessage != nil {
		span.SetAttributes(tracer.AttrOwnerID.String(e.NewMessage.OwnerID))
	}

	if err = util.Validate(e.NewMessage); err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	var isDuplicate bool
	idempotencyStart := time.Now()
	if !util.IsStringEmpty(e.NewMessage.IdempotencyKey) {
		// Optimization: Use FindFirstEventWithIdempotencyKey (LIMIT 1) instead of FindEventsByIdempotencyKey (returns all)
		// We only need to check if an event exists, not fetch all matching events
		_, err := e.EventRepo.FindFirstEventWithIdempotencyKey(ctx, e.Project.UID, e.NewMessage.IdempotencyKey)
		if err != nil && !errors.Is(err, datastore.ErrEventNotFound) {
			e.Logger.ErrorContext(ctx, "failed to check idempotency key", "error", err)
			return nil, &ServiceError{ErrMsg: err.Error()}
		}
		isDuplicate = (err == nil)
		e.Logger.DebugContext(ctx, "idempotency check completed", "duration", time.Since(idempotencyStart), "found", isDuplicate)
	}
	afterIdempotency := time.Now()

	endpointIDs, err := e.EndpointRepo.FetchEndpointIDsByOwnerID(ctx, e.Project.UID, e.NewMessage.OwnerID)
	if err != nil {
		e.Logger.ErrorContext(ctx, "failed to find endpoints by owner id", "error", err)
		return nil, &ServiceError{ErrMsg: err.Error()}
	}
	afterEndpoints := time.Now()
	e.Logger.DebugContext(ctx, "endpoint lookup completed", "duration", afterEndpoints.Sub(afterIdempotency), "endpoints", len(endpointIDs))

	afterPortalLink := afterEndpoints
	if len(endpointIDs) == 0 {
		_, err = e.PortalLinkRepo.GetPortalLinkByOwnerID(ctx, e.Project.UID, e.NewMessage.OwnerID)
		if err != nil {
			if !errors.Is(err, datastore.ErrPortalLinkNotFound) {
				e.Logger.ErrorContext(ctx, "failed to find portal link by owner id", "error", err)
				return nil, &ServiceError{ErrMsg: err.Error()}
			}
		}
		afterPortalLink = time.Now()
		e.Logger.DebugContext(ctx, "portal link lookup completed", "duration", afterPortalLink.Sub(afterEndpoints), "found", err == nil)
	}

	ev := &newEvent{
		UID:            ulid.Make().String(),
		Data:           e.NewMessage.Data,
		EventType:      e.NewMessage.EventType,
		IdempotencyKey: e.NewMessage.IdempotencyKey,
		Raw:            "", // Skip Raw duplication - Data field is canonical (saves ~629KB per event)
		CustomHeaders:  e.NewMessage.CustomHeaders,
		IsDuplicate:    isDuplicate,
		AcknowledgedAt: time.Now(),
	}

	event, err = createEvent(ctx, endpointIDs, ev, e.Project, e.Queue, e.Logger)
	afterQueue := time.Now()
	if err != nil {
		e.Logger.ErrorContext(ctx, "failed to create fanout event", "error", err)
		return nil, err
	}

	// Log detailed service timing breakdown for performance monitoring
	e.Logger.InfoContext(ctx, "fanout service timing breakdown",
		"idempotency_duration", afterIdempotency.Sub(idempotencyStart).Milliseconds(),
		"endpoints_duration", afterEndpoints.Sub(afterIdempotency).Milliseconds(),
		"portal_link_duration", afterPortalLink.Sub(afterEndpoints).Milliseconds(),
		"queue_duration", afterQueue.Sub(afterPortalLink).Milliseconds(),
		"total_service_duration", afterQueue.Sub(serviceStart).Milliseconds(),
		"event_id", event.UID,
		"endpoints_count", len(endpointIDs),
	)

	return event, nil
}

func createEvent(ctx context.Context, endpointIDs []string, newMessage *newEvent, project *datastore.Project, queuer queue.Queuer, logger log.Logger) (*datastore.Event, error) {
	jobId := queue.JobId{ProjectID: project.UID, ResourceID: newMessage.UID}.FanOutJobId()
	event := &datastore.Event{
		UID:              newMessage.UID,
		EventType:        datastore.EventType(newMessage.EventType),
		Data:             newMessage.Data,
		Raw:              newMessage.Raw,
		IdempotencyKey:   newMessage.IdempotencyKey,
		IsDuplicateEvent: newMessage.IsDuplicate,
		Headers:          getCustomHeaders(newMessage.CustomHeaders),
		Endpoints:        endpointIDs,
		ProjectID:        project.UID,
		AcknowledgedAt:   null.TimeFrom(time.Now()),
	}

	if (project.Config == nil || project.Config.Strategy == nil) ||
		(project.Config.Strategy != nil && project.Config.Strategy.Type != datastore.LinearStrategyProvider && project.Config.Strategy.Type != datastore.ExponentialStrategyProvider) {
		return nil, &ServiceError{ErrMsg: "retry strategy not defined in configuration"}
	}

	e := task.CreateEvent{
		JobID:              jobId,
		Event:              event,
		CreateSubscription: !util.IsStringEmpty(newMessage.EndpointID),
	}

	eventByte, err := msgpack.EncodeMsgPack(e)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	job := &queue.Job{
		ID:      jobId,
		Payload: eventByte,
	}
	startQueue := time.Now()
	err = queuer.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
	if err != nil {
		logger.ErrorContext(ctx, fmt.Sprintf("Error occurred sending new event to the queue %s", err))
		return nil, &ServiceError{ErrMsg: err.Error()}
	}
	logger.DebugContext(ctx, "event written to queue", "duration", time.Since(startQueue))

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
