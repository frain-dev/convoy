package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/dynamiceventack"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

type CreateDynamicEventService struct {
	Queue queue.Queuer
	Redis redis.UniversalClient

	DynamicEvent *models.DynamicEvent
	Project      *datastore.Project
	Logger       log.Logger
}

func (e *CreateDynamicEventService) Run(ctx context.Context) (err error) {
	ctx, span := otel.Tracer(tracer.TracerNameServices).Start(ctx, tracer.SpanServicesEventCreateDynamic)
	defer func() {
		tracer.RecordError(span, err)
		span.End()
	}()

	if e.Project == nil {
		return &ServiceError{ErrMsg: "an error occurred while creating dynamic event - invalid project"}
	}
	span.SetAttributes(tracer.AttrProjectID.String(e.Project.UID))

	if err := e.Project.ValidateOutgoingEventIdempotencyKey(e.DynamicEvent.IdempotencyKey); err != nil {
		return &ServiceError{ErrMsg: err.Error()}
	}

	id := ulid.Make().String()
	jobId := queue.JobId{ProjectID: e.Project.UID, ResourceID: id}.DynamicJobId()

	e.DynamicEvent.EventID = id
	e.DynamicEvent.JobID = jobId
	e.DynamicEvent.ProjectID = e.Project.UID
	e.DynamicEvent.AcknowledgedAt = time.Now()

	// Do not default EventTypes to ["*"] here. An empty list must reach the worker as
	// empty so it can distinguish "caller did not specify a filter" (leave an existing
	// subscription's filter untouched, and default only a brand-new subscription to
	// catch-all) from "caller explicitly set a filter" (sync it onto the subscription).
	// Defaulting here would make every dynamic event that omits event_types silently
	// overwrite the matched subscription's filter with ["*"].

	taskName := convoy.CreateDynamicEventProcessor

	eventByte, err := msgpack.EncodeMsgPack(e.DynamicEvent)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, err)
	}

	job := &queue.Job{
		ID:      jobId,
		Payload: eventByte,
	}

	err = e.Queue.Write(ctx, taskName, convoy.CreateEventQueue, job)
	if err != nil {
		e.Logger.ErrorContext(ctx, fmt.Sprintf("Error occurred sending new dynamic event to the queue %s", err))
		return &ServiceError{ErrMsg: "failed to create dynamic event"}
	}

	if e.Project.Config == nil || !e.Project.Config.VerifyDynamicEvents {
		return nil
	}

	// Failure policy: fail closed. Timeout / redis / resolve errors must not return 201.
	if e.Redis == nil {
		return util.NewServiceError(http.StatusServiceUnavailable, dynamiceventack.ErrNilRedis)
	}

	cfg, cfgErr := config.Get()
	if cfgErr != nil {
		return util.NewServiceError(http.StatusInternalServerError, cfgErr)
	}
	timeout := time.Duration(cfg.VerifyDynamicEventsTimeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	result, waitErr := dynamiceventack.Wait(ctx, e.Redis, e.Project.UID, id, timeout)
	if waitErr != nil {
		if errors.Is(waitErr, dynamiceventack.ErrTimeout) {
			return util.NewServiceError(http.StatusGatewayTimeout, waitErr)
		}
		return util.NewServiceError(http.StatusServiceUnavailable, waitErr)
	}
	if !result.OK {
		msg := result.Error
		if msg == "" {
			msg = "dynamic event resolve failed"
		}
		return util.NewServiceError(http.StatusBadRequest, errors.New(msg))
	}

	return nil
}
