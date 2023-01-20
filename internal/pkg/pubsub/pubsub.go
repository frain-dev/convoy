package pubsub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/google"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/sqs"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PubSub interface {
	Dispatch()
	Listen()
	Stop()
}

type Source struct {
	// The pub sub client
	client PubSub

	// An identifier for the source config
	hash string
}

type SourcePool struct {
	queue   queue.Queuer
	sources map[string]*Source
}

var (
	handlerFunc = func(source *datastore.Source, q queue.Queuer, msg string) error {
		var ev models.Event

		if err := json.Unmarshal([]byte(msg), &ev); err != nil {
			return err
		}

		event := datastore.Event{
			UID:       uuid.NewString(),
			EventType: datastore.EventType(ev.EventType),
			SourceID:  source.UID,
			ProjectID: source.ProjectID,
			Raw:       string(ev.Data),
			Data:      ev.Data,
			CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
			Endpoints: []string{ev.EndpointID},
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

		err = q.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
		if err != nil {
			return err
		}

		return nil
	}
)

func NewSourcePool(queue queue.Queuer) *SourcePool {
	return &SourcePool{
		queue:   queue,
		sources: make(map[string]*Source),
	}
}

func (s *SourcePool) Insert(source *datastore.Source) error {
	// Make sure the source doesn't already exists in the source
	// pool. If it does, ensure the hash hasn't changed
	sour, exists := s.sources[source.UID]
	if exists {
		// The source config has changed
		if s.hash(source) != sour.hash {
			s.Remove(source.UID)
			return s.insert(source)
		}

		return nil
	}

	return s.insert(source)
}

func (s *SourcePool) insert(source *datastore.Source) error {
	client, err := NewPubSub(source, s.queue)
	if err != nil {
		return err
	}

	client.Dispatch()
	sourceSteam := &Source{
		client: client,
		hash:   s.hash(source),
	}

	s.sources[source.UID] = sourceSteam
	return nil
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

func (s *SourcePool) hash(source *datastore.Source) string {
	var hash string

	if source.PubSubConfig.Type == datastore.SqsPubSub {
		sq := source.PubSubConfig.Sqs
		hash = fmt.Sprintf("%s,%s,%s,%s,%v", sq.AccessKeyID, sq.SecretKey, sq.DefaultRegion, sq.QueueName, source.PubSubConfig.Workers)
	}

	if source.PubSubConfig.Type == datastore.GooglePubSub {
		gq := source.PubSubConfig.Google
		hash = fmt.Sprintf("%s,%s,%s,%v", gq.ApiKey, gq.ProjectID, gq.TopicName, source.PubSubConfig.Workers)
	}

	return base64.StdEncoding.EncodeToString([]byte(hash))
}

func NewPubSub(source *datastore.Source, queue queue.Queuer) (PubSub, error) {
	if source.PubSubConfig.Type == datastore.SqsPubSub {
		return sqs.New(source, queue, handlerFunc), nil
	}

	if source.PubSubConfig.Type == datastore.GooglePubSub {
		return google.New(source), nil
	}

	return nil, fmt.Errorf("pub sub type %s is not supported", source.PubSubConfig.Type)
}
