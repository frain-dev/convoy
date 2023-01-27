package pubsub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/google"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/sqs"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PubSub interface {
	Start()
	Consume()
	Stop()
}

type Source struct {
	// The pub sub client.
	client PubSub

	// This is an identifier for the source config used to
	// track if an existing source config has been changed.
	hash string
}

type SourcePool struct {
	queue        queue.Queuer
	sourceRepo   datastore.SourceRepository
	endpointRepo datastore.EndpointRepository
	sources      map[string]*Source
}

func NewSourcePool(queue queue.Queuer, sourceRepo datastore.SourceRepository, endpointRepo datastore.EndpointRepository) *SourcePool {
	return &SourcePool{
		queue:        queue,
		sourceRepo:   sourceRepo,
		endpointRepo: endpointRepo,
		sources:      make(map[string]*Source),
	}
}

func (s *SourcePool) Insert(source *datastore.Source, client PubSub) {
	existingSource, exists := s.sources[source.UID]
	if !exists {
		s.insert(source, client)
		return
	}

	// The source config has changed
	if s.hash(source) != existingSource.hash {
		s.Remove(source.UID)
		s.insert(source, client)
	}
}

func (s *SourcePool) insert(source *datastore.Source, client PubSub) {
	client.Start()

	sourceSteam := &Source{
		client: client,
		hash:   s.hash(source),
	}

	s.sources[source.UID] = sourceSteam
}

func (s *SourcePool) Remove(sourceId string) {
	s.sources[sourceId].client.Stop()
	delete(s.sources, sourceId)
}

func (s *SourcePool) Stop() {
	for key, source := range s.sources {
		log.Printf("Stopping pub source with ID: %s", key)
		source.client.Stop()
	}
}

func (s *SourcePool) FetchSources(page int) error {
	filter := &datastore.SourceFilter{
		Type: string(datastore.PubSubSource),
	}

	pageable := datastore.Pageable{
		Page:    page,
		PerPage: 50,
	}

	sources, _, err := s.sourceRepo.LoadSourcesPaged(context.Background(), "", filter, pageable)
	if err != nil {
		return err
	}

	if len(sources) == 0 {
		return nil
	}

	for _, source := range sources {
		client, err := NewPubSub(&source, s.handler)
		if err != nil {
			log.WithError(err).Error("failed to create pub sub client")
			continue
		}

		s.Insert(&source, client)
	}

	page += 1
	return s.FetchSources(page)
}

func (s *SourcePool) handler(source *datastore.Source, msg string) error {
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
		endpoint, err := s.endpointRepo.FindEndpointByID(context.Background(), ev.EndpointID)
		if err != nil {
			return err
		}

		endpoints = append(endpoints, endpoint.UID)
	}

	event := datastore.Event{
		UID:       uuid.NewString(),
		EventType: datastore.EventType(ev.EventType),
		SourceID:  source.UID,
		ProjectID: source.ProjectID,
		Raw:       string(ev.Data),
		Data:      ev.Data,
		Headers:   getCustomHeaders(ev.CustomHeaders),
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
		Endpoints: endpoints,
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

func (s *SourcePool) hash(source *datastore.Source) string {
	var hash string

	if source.PubSub.Type == datastore.SqsPubSub {
		sq := source.PubSub.Sqs
		hash = fmt.Sprintf("%s,%s,%s,%s,%v", sq.AccessKeyID, sq.SecretKey, sq.DefaultRegion, sq.QueueName, source.PubSub.Workers)
	}

	if source.PubSub.Type == datastore.GooglePubSub {
		gq := source.PubSub.Google
		hash = fmt.Sprintf("%s,%s,%s,%v", gq.ServiceAccount, gq.ProjectID, gq.SubscriptionID, source.PubSub.Workers)
	}

	return base64.StdEncoding.EncodeToString([]byte(hash))
}

func NewPubSub(source *datastore.Source, handler datastore.PubSubHandler) (PubSub, error) {
	if source.PubSub.Type == datastore.SqsPubSub {
		return sqs.New(source, handler), nil
	}

	if source.PubSub.Type == datastore.GooglePubSub {
		return google.New(source, handler), nil
	}

	return nil, fmt.Errorf("pub sub type %s is not supported", source.PubSub.Type)
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
