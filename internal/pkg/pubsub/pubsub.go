package pubsub

import (
	"encoding/base64"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/google"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/sqs"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
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
		return sqs.New(source, queue), nil
	}

	if source.PubSubConfig.Type == datastore.GooglePubSub {
		return google.New(source), nil
	}

	return nil, fmt.Errorf("pub sub type %s is not supported", source.PubSubConfig.Type)
}
