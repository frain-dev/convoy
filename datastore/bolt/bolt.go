package bolt

import (
	"context"

	"github.com/dgraph-io/badger/v3"
	"github.com/timshannon/badgerhold/v4"

	"go.etcd.io/bbolt"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

type Client struct {
	store             *badgerhold.Store
	db                *bbolt.DB
	apiKeyRepo        datastore.APIKeyRepository
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	applicationRepo   datastore.ApplicationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
}

func New(cfg config.Configuration) (datastore.DatabaseClient, error) {
	st, err := badgerhold.Open(badgerhold.Options{
		Encoder:          badgerhold.DefaultEncode,
		Decoder:          badgerhold.DefaultDecode,
		SequenceBandwith: 100,
		Options: badger.DefaultOptions("convoy_tmp_db").
			WithZSTDCompressionLevel(0).
			WithCompression(0),
	})
	if err != nil {
		return nil, err
	}

	db, err := bbolt.Open(cfg.Database.Dsn, 0666, nil)
	if err != nil {
		return nil, err
	}

	c := &Client{
		store:             st,
		db:                db,
		groupRepo:         NewGroupRepo(st),
		eventRepo:         NewEventRepo(db),
		apiKeyRepo:        NewApiRoleRepo(st),
		applicationRepo:   NewApplicationRepo(st),
		eventDeliveryRepo: NewEventDeliveryRepository(st),
	}

	return c, nil
}

func (c *Client) Disconnect(ctx context.Context) error {
	return c.store.Close()
}

func (c *Client) GetName() string {
	return "bolt"
}

func (c *Client) Client() interface{} {
	return c.store
}

func (c *Client) GroupRepo() datastore.GroupRepository {
	return c.groupRepo
}

func (c *Client) AppRepo() datastore.ApplicationRepository {
	return c.applicationRepo
}

func (c *Client) EventRepo() datastore.EventRepository {
	return c.eventRepo
}

func (c *Client) EventDeliveryRepo() datastore.EventDeliveryRepository {
	return c.eventDeliveryRepo
}

func (c *Client) APIRepo() datastore.APIKeyRepository {
	return c.apiKeyRepo
}
