package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy/internal/pkg/apm"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/oklog/ulid/v2"
)

const (
	perPage = 50
)

type SourceLoader struct {
	endpointRepo datastore.EndpointRepository
	sourceRepo   datastore.SourceRepository
	projectRepo  datastore.ProjectRepository
	eventRepo    datastore.EventRepository
	queue        queue.Queuer
	sourcePool   *SourcePool
	log          log.StdLogger
}

func NewSourceLoader(eventRepo datastore.EventRepository, endpointRepo datastore.EndpointRepository, sourceRepo datastore.SourceRepository, projectRepo datastore.ProjectRepository, queue queue.Queuer, sourcePool *SourcePool, log log.StdLogger) *SourceLoader {
	return &SourceLoader{
		eventRepo:    eventRepo,
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

func (s *SourceLoader) fetchSources(ctx context.Context, projectID string, cursor string) error {
	txn, innerCtx := apm.StartTransaction(ctx, "fetchSources")
	defer txn.End()

	filter := &datastore.SourceFilter{
		Type: string(datastore.PubSubSource),
	}

	pageable := datastore.Pageable{
		NextCursor: cursor,
		Direction:  datastore.Next,
		PerPage:    perPage,
	}

	sources, pagination, err := s.sourceRepo.LoadSourcesPaged(innerCtx, projectID, filter, pageable)
	if err != nil {
		return err
	}

	if len(sources) == 0 && !pagination.HasNextPage {
		return nil
	}

	for _, source := range sources {
		go func(source datastore.Source) {
			ps, err := NewPubSubSource(&source, s.handler, s.log)
			if err != nil {
				s.log.WithError(err).Error("failed to create pub sub source")
			}

			s.sourcePool.Insert(ps)
		}(source)
	}

	cursor = pagination.NextPageCursor
	return s.fetchSources(innerCtx, projectID, cursor)
}

func (s *SourceLoader) fetchProjectSources(ctx context.Context) error {
	txn, innerCtx := apm.StartTransaction(ctx, "fetchProjectSources")
	defer txn.End()

	projects, err := s.projectRepo.LoadProjects(innerCtx, &datastore.ProjectFilter{})
	if err != nil {
		return err
	}

	for _, project := range projects {
		err := s.fetchSources(innerCtx, project.UID, fmt.Sprintf("%d", math.MaxInt))
		if err != nil {
			s.log.WithError(err).Error("failed to load sources")
			continue
		}
	}

	return nil
}

func (s *SourceLoader) handler(ctx context.Context, source *datastore.Source, msg string) error {
	txn, innerCtx := apm.StartTransaction(ctx, fmt.Sprintf("%v handler", source.Name))
	defer txn.End()

	ev := struct {
		EndpointID     string            `json:"endpoint_id"`
		OwnerID        string            `json:"owner_id"`
		EventType      string            `json:"event_type"`
		Data           json.RawMessage   `json:"data"`
		IdempotencyKey string            `json:"idempotency_key"`
		CustomHeaders  map[string]string `json:"custom_headers"`
	}{}

	if err := json.Unmarshal([]byte(msg), &ev); err != nil {
		return err
	}

	var IsDuplicate bool
	if !util.IsStringEmpty(ev.IdempotencyKey) {
		events, err := s.eventRepo.FindEventsByIdempotencyKey(ctx, source.ProjectID, ev.IdempotencyKey)
		if err != nil {
			return err
		}

		IsDuplicate = len(events) > 0
	}

	var endpoints []string

	if !util.IsStringEmpty(ev.OwnerID) {
		ownerIdEndpoints, err := s.endpointRepo.FindEndpointsByOwnerID(innerCtx, source.ProjectID, ev.OwnerID)
		if err != nil {
			return err
		}

		if len(ownerIdEndpoints) == 0 {
			return errors.New("owner ID has no configured endpoints")
		}

		for _, endpoint := range ownerIdEndpoints {
			endpoints = append(endpoints, endpoint.UID)
		}
	} else {
		endpoint, err := s.endpointRepo.FindEndpointByID(innerCtx, ev.EndpointID, source.ProjectID)
		if err != nil {
			return err
		}

		endpoints = append(endpoints, endpoint.UID)
	}

	event := datastore.Event{
		UID:              ulid.Make().String(),
		EventType:        datastore.EventType(ev.EventType),
		SourceID:         source.UID,
		ProjectID:        source.ProjectID,
		Raw:              string(ev.Data),
		Data:             ev.Data,
		IdempotencyKey:   ev.IdempotencyKey,
		IsDuplicateEvent: IsDuplicate,
		Headers:          getCustomHeaders(ev.CustomHeaders),
		Endpoints:        endpoints,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	createEvent := task.CreateEvent{
		Event:              event,
		CreateSubscription: !util.IsStringEmpty(ev.EndpointID),
	}

	eventByte, err := msgpack.EncodeMsgPack(createEvent)
	if err != nil {
		return err
	}

	job := &queue.Job{
		ID:      event.UID,
		Payload: eventByte,
		Delay:   0,
	}

	err = s.queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
	if err != nil {
		return err
	}

	return nil
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
