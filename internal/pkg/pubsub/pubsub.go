package pubsub

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/google"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/sqs"
	"github.com/frain-dev/convoy/pkg/log"
)

type PubSub interface {
	Start()
	Consume()
	Stop()
}

type PubSubSource struct {
	// The pub sub client.
	client PubSub

	// The DB source
	source *datastore.Source

	// This is a hash for the source config used to
	// track if an existing source config has been changed.
	hash string
}

func NewPubSubSource(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) (*PubSubSource, error) {
	client, err := NewPubSubClient(source, handler, log)
	if err != nil {
		return nil, err
	}

	pubSubSource := &PubSubSource{client: client, source: source}
	pubSubSource.hash = pubSubSource.getHash()
	return pubSubSource, nil
}

func (p *PubSubSource) Start() {
	p.client.Start()
}

func (p *PubSubSource) Stop() {
	p.client.Stop()
}

func (p *PubSubSource) getHash() string {
	var hash string

	source := p.source
	if source.PubSub.Type == datastore.SqsPubSub {
		sq := source.PubSub.Sqs
		hash = fmt.Sprintf("%s,%s,%s,%s,%v", sq.AccessKeyID, sq.SecretKey, sq.DefaultRegion, sq.QueueName, source.PubSub.Workers)
	}

	if source.PubSub.Type == datastore.GooglePubSub {
		gq := source.PubSub.Google
		hash = fmt.Sprintf("%s,%s,%s,%v", gq.ServiceAccount, gq.ProjectID, gq.SubscriptionID, source.PubSub.Workers)
	}

	h := md5.Sum([]byte(hash))
	hash = hex.EncodeToString(h[:])

	return hash

}

type SourcePool struct {
	log     log.StdLogger
	sources map[string]*PubSubSource
}

func NewSourcePool(log log.StdLogger) *SourcePool {
	return &SourcePool{
		log:     log,
		sources: make(map[string]*PubSubSource),
	}
}

func (s *SourcePool) Insert(ps *PubSubSource) error {
	source := ps.source
	existingSource, exists := s.sources[source.UID]

	if exists {
		so := &PubSubSource{source: source}
		// config hasn't changed
		if existingSource.hash == so.getHash() {
			return nil
		}

		s.Remove(source.UID)
	}

	ps.Start()
	s.sources[source.UID] = ps
	return nil
}

func (s *SourcePool) Remove(sourceId string) {
	s.sources[sourceId].Stop()
	delete(s.sources, sourceId)
}

func (s *SourcePool) Stop() {
	for key, source := range s.sources {
		s.log.Infof("Stopping pub source with ID: %s", key)
		source.Stop()
	}
}

func NewPubSubClient(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) (PubSub, error) {
	if source.PubSub.Type == datastore.SqsPubSub {
		return sqs.New(source, handler, log), nil
	}

	if source.PubSub.Type == datastore.GooglePubSub {
		return google.New(source, handler, log), nil
	}

	return nil, fmt.Errorf("pub sub type %s is not supported", source.PubSub.Type)
}
