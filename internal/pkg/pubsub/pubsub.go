package pubsub

import (
	"encoding/base64"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/google"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/sqs"
)

type PubSub interface {
	Dispatch()
	Listen()
	Stop()
}

type SourceStream struct {
	client PubSub
	hash   string
}

type SourcePool struct {
	sources map[string]*SourceStream
}

func NewSourcePool() *SourcePool {
	return &SourcePool{
		sources: make(map[string]*SourceStream),
	}
}

func (s *SourcePool) Insert(source *datastore.Source) error {
	// before inserting, we need to make sure the source doesn't already exists in the source
	// pool. If it does, make sure the hash hasn't changed
	sour, exists := s.sources[source.UID]
	if exists {
		// The source config has changed
		if s.hash(source) != sour.hash {
			s.Remove(source.UID)
			s.insert(source)
		}

		return nil
	}

	return s.insert(source)
}

func (s *SourcePool) insert(source *datastore.Source) error {
	client, err := NewPubSub(source.PubSubConfig)
	if err != nil {
		return err
	}

	client.Dispatch()
	sourceSteam := &SourceStream{
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

func NewPubSub(cfg *datastore.PubSubConfig) (PubSub, error) {
	if cfg.Type == datastore.SqsPubSub {
		return sqs.New(cfg.Sqs, cfg.Workers), nil
	}

	if cfg.Type == datastore.GooglePubSub {
		return google.New(cfg.Google, cfg.Workers), nil
	}

	return nil, fmt.Errorf("pub sub type %s is not supported", cfg.Type)
}
