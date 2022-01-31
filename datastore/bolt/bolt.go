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
	dbh               *badgerhold.Store
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
		Options: badger.DefaultOptions("tmp").
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
		dbh:               st,
		db:                db,
		groupRepo:         NewGroupRepo(db),
		eventRepo:         NewEventRepo(db),
		apiKeyRepo:        NewApiRoleRepo(db),
		applicationRepo:   NewApplicationRepo(db),
		eventDeliveryRepo: NewEventDeliveryRepository(db),
	}

	return c, nil
}

func (c *Client) Disconnect(ctx context.Context) error {
	return c.db.Close()
}

func (c *Client) GetName() string {
	return "bolt"
}

func (c *Client) Client() interface{} {
	return c.db
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
