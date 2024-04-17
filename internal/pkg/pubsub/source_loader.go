package pubsub

import (
	"context"
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy/worker/task"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

const (
	perPage = 50
)

type SourceLoader struct {
	endpointRepo datastore.EndpointRepository
	sourceRepo   datastore.SourceRepository
	projectRepo  datastore.ProjectRepository
	queue        queue.Queuer
	sourcePool   *SourcePool
	log          log.StdLogger
}

func NewSourceLoader(endpointRepo datastore.EndpointRepository, sourceRepo datastore.SourceRepository, projectRepo datastore.ProjectRepository, queue queue.Queuer, sourcePool *SourcePool, log log.StdLogger) *SourceLoader {
	return &SourceLoader{
		endpointRepo: endpointRepo,
		sourceRepo:   sourceRepo,
		projectRepo:  projectRepo,
		queue:        queue,
		sourcePool:   sourcePool,
		log:          log,
	}
}

func (s *SourceLoader) Run(ctx context.Context, interval int, stop <-chan struct{}) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)

	for {
		select {
		case <-ticker.C:
			err := s.fetchProjectSources(ctx)
			if err != nil {
				s.log.WithError(err).Error("failed to fetch sources")
			}
		case <-stop:
			// Stop the ticker
			ticker.Stop()

			// Stop the existing pub sub sources
			s.sourcePool.Stop()
			return
		}
	}
}

func (s *SourceLoader) fetchSources(ctx context.Context, projectIDs []string, cursor string) error {
	pageable := datastore.Pageable{
		NextCursor: cursor,
		Direction:  datastore.Next,
		PerPage:    perPage,
	}

	sources, pagination, err := s.sourceRepo.LoadPubSubSourcesByProjectIDs(ctx, projectIDs, pageable)
	if err != nil {
		return err
	}

	if len(sources) == 0 && !pagination.HasNextPage {
		return nil
	}

	for i := range sources {
		ps, err := NewPubSubSource(&sources[i], s.handler, s.log)
		if err != nil {
			s.log.WithError(err).Error("failed to create pub sub source")
		}

		s.sourcePool.Insert(ps)
	}

	if pagination.HasNextPage {
		cursor = pagination.NextPageCursor
		return s.fetchSources(ctx, projectIDs, cursor)
	}

	return nil
}

func (s *SourceLoader) fetchProjectSources(ctx context.Context) error {
	projects, err := s.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return err
	}

	ids := make([]string, len(projects))
	for i := range projects {
		ids[i] = projects[i].UID
	}

	err = s.fetchSources(ctx, ids, "")
	if err != nil {
		s.log.WithError(err).Error("failed to load sources")
		return err
	}

	return nil
}

func (s *SourceLoader) handler(_ context.Context, source *datastore.Source, msg string) error {
	ev := struct {
		EndpointID     string            `json:"endpoint_id"`
		OwnerID        string            `json:"owner_id"`
		EventType      string            `json:"event_type"`
		Data           json.RawMessage   `json:"data"`
		CustomHeaders  map[string]string `json:"custom_headers"`
		IdempotencyKey string            `json:"idempotency_key"`
	}{}

	if err := json.Unmarshal([]byte(msg), &ev); err != nil {
		return err
	}

	ce := task.CreateEvent{
		Params: task.CreateEventTaskParams{
			UID:            ulid.Make().String(),
			SourceID:       source.UID,
			ProjectID:      source.ProjectID,
			OwnerID:        ev.OwnerID,
			EndpointID:     ev.EndpointID,
			EventType:      ev.EventType,
			Data:           ev.Data,
			CustomHeaders:  ev.CustomHeaders,
			IdempotencyKey: ev.IdempotencyKey,
		},
		CreateSubscription: !util.IsStringEmpty(ev.EndpointID),
	}

	eventByte, err := msgpack.EncodeMsgPack(ce)
	if err != nil {
		return err
	}

	job := &queue.Job{
		ID:      ce.Params.UID,
		Payload: eventByte,
		Delay:   0,
	}

	err = s.queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
	if err != nil {
		return err
	}

	return nil
}
