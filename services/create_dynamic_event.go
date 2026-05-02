package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
	"go.opentelemetry.io/otel"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

type CreateDynamicEventService struct {
	Queue queue.Queuer

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
	id := ulid.Make().String()
	jobId := queue.JobId{ProjectID: e.Project.UID, ResourceID: id}.DynamicJobId()

	e.DynamicEvent.EventID = id
	e.DynamicEvent.JobID = jobId
	e.DynamicEvent.ProjectID = e.Project.UID
	e.DynamicEvent.AcknowledgedAt = time.Now()

	if len(e.DynamicEvent.EventTypes) == 0 {
		e.DynamicEvent.EventTypes = []string{"*"}
	}

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

	return nil
}
