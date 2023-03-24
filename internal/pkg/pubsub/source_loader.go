package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"time"

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
	queue        queue.Queuer
	sourcePool   *SourcePool
	log          log.StdLogger
}

func NewSourceLoader(endpointRepo datastore.EndpointRepository, sourceRepo datastore.SourceRepository, queue queue.Queuer, sourcePool *SourcePool, log log.StdLogger) *SourceLoader {
	return &SourceLoader{
		endpointRepo: endpointRepo,
		sourceRepo:   sourceRepo,
		queue:        queue,
		sourcePool:   sourcePool,
		log:          log,
	}
}

func (s *SourceLoader) Run(interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	exit := make(chan os.Signal, 1)

	signal.Notify(exit, os.Interrupt)

	for {
		select {
		case <-ticker.C:
			page := 1
			err := s.fetchSources(page)
			if err != nil {
				s.log.WithError(err).Error("failed to fetch sources")
			}

		case <-exit:
			// Stop the ticker
			ticker.Stop()

			// Stop the existing pub sub sources
			s.sourcePool.Stop()
			return
		}
	}
}

func (s *SourceLoader) fetchSources(page int) error {
	filter := &datastore.SourceFilter{
		Type: string(datastore.PubSubSource),
	}

	pageable := datastore.Pageable{
		PerPage: perPage,
	}

	sources, _, err := s.sourceRepo.LoadSourcesPaged(context.Background(), "", filter, pageable)
	if err != nil {
		return err
	}

	if len(sources) == 0 {
		return nil
	}

	for _, source := range sources {
		ps, err := NewPubSubSource(&source, s.handler, s.log)
		if err != nil {
			s.log.WithError(err).Error("failed to create pub sub source")
		}

		s.sourcePool.Insert(ps)
	}

	page += 1
	return s.fetchSources(page)
}

func (s *SourceLoader) handler(source *datastore.Source, msg string) error {
	ev := struct {
		EndpointID    string            `json:"endpoint_id"`
		OwnerID       string            `json:"owner_id"`
		EventType     string            `json:"event_type"`
		Data          json.RawMessage   `json:"data"`
		CustomHeaders map[string]string `json:"custom_headers"`
	}{}

	if err := json.Unmarshal([]byte(msg), &ev); err != nil {
		return err
	}

	var endpoints []string

	if !util.IsStringEmpty(ev.OwnerID) {
		ownerIdEndpoints, err := s.endpointRepo.FindEndpointsByOwnerID(context.Background(), source.ProjectID, ev.OwnerID)
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
		endpoint, err := s.endpointRepo.FindEndpointByID(context.Background(), ev.EndpointID, source.ProjectID)
		if err != nil {
			return err
		}

		endpoints = append(endpoints, endpoint.UID)
	}

	event := datastore.Event{
		UID:       ulid.Make().String(),
		EventType: datastore.EventType(ev.EventType),
		SourceID:  source.UID,
		ProjectID: source.ProjectID,
		Raw:       string(ev.Data),
		Data:      ev.Data,
		Headers:   getCustomHeaders(ev.CustomHeaders),
		Endpoints: endpoints,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	createEvent := task.CreateEvent{
		Event:              event,
		CreateSubscription: !util.IsStringEmpty(ev.EndpointID),
	}

	eventByte, err := json.Marshal(createEvent)
	if err != nil {
		return err
	}

	job := &queue.Job{
		ID:      event.UID,
		Payload: json.RawMessage(eventByte),
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
