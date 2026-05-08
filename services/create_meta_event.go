package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/oklog/ulid/v2"
	"go.opentelemetry.io/otel"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
)

type MetaEvent struct {
	queue         queue.Queuer
	projectRepo   datastore.ProjectRepository
	metaEventRepo datastore.MetaEventRepository
	logger        log.Logger
}

func NewMetaEvent(queue queue.Queuer, projectRepo datastore.ProjectRepository, metaEventRepo datastore.MetaEventRepository, logger log.Logger) *MetaEvent {
	return &MetaEvent{queue: queue, projectRepo: projectRepo, metaEventRepo: metaEventRepo, logger: logger}
}

func (m *MetaEvent) Run(ctx context.Context, eventType, projectID string, data interface{}) (err error) {
	ctx, span := otel.Tracer(tracer.TracerNameServices).Start(ctx, tracer.SpanServicesEventCreateMeta)
	span.SetAttributes(tracer.AttrProjectID.String(projectID))
	defer func() {
		tracer.RecordError(span, err)
		span.End()
	}()

	project, err := m.projectRepo.FetchProjectByID(ctx, projectID)
	if err != nil {
		return err
	}

	cfg := project.Config
	if cfg.MetaEvent == nil {
		return nil
	}

	if !cfg.MetaEvent.IsEnabled {
		return nil
	}

	if !m.isSubscribed(eventType, cfg.MetaEvent.EventType) {
		return nil
	}

	dByte, err := json.Marshal(data)
	if err != nil {
		return err
	}

	mP := datastore.MetaEventPayload{
		EventType: eventType,
		Data:      dByte,
	}

	mpByte, err := json.Marshal(mP)
	if err != nil {
		return err
	}

	metaData := &datastore.Metadata{
		NumTrials:       0,
		RetryLimit:      project.Config.Strategy.RetryCount,
		Data:            mpByte,
		Raw:             "", // Skip Raw duplication - Data field is canonical (reduces payload size)
		IntervalSeconds: project.Config.Strategy.Duration,
		Strategy:        project.Config.Strategy.Type,
		NextSendTime:    time.Now(),
	}

	metaEvent := &datastore.MetaEvent{
		UID:       ulid.Make().String(),
		ProjectID: projectID,
		EventType: eventType,
		Status:    datastore.ScheduledEventStatus,
		Metadata:  metaData,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = m.metaEventRepo.CreateMetaEvent(ctx, metaEvent)
	if err != nil {
		m.logger.Error("failed to create meta event", "error", err)
		return err
	}

	s := task.MetaEvent{
		MetaEventID: metaEvent.UID,
		ProjectID:   projectID,
	}

	bytes, err := msgpack.EncodeMsgPack(s)
	if err != nil {
		return err
	}

	jobId := queue.JobId{ProjectID: metaEvent.ProjectID, ResourceID: metaEvent.UID}.MetaJobId()
	err = m.queue.Write(ctx, convoy.MetaEventProcessor, convoy.MetaEventQueue, &queue.Job{
		ID:      jobId,
		Payload: bytes,
	})

	if err != nil {
		return err
	}

	return nil
}

func (m *MetaEvent) isSubscribed(eventType string, events []string) bool {
	for _, event := range events {
		if event == eventType {
			return true
		}
	}

	return false
}
