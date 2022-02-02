package badger

import (
	"context"
	"encoding/json"
	"io"

	"github.com/frain-dev/convoy/util"

	"github.com/dgraph-io/badger/v3"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/sirupsen/logrus"
	"github.com/timshannon/badgerhold/v4"
)

type Client struct {
	store             *badgerhold.Store
	apiKeyRepo        datastore.APIKeyRepository
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	applicationRepo   datastore.ApplicationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
}

func New(cfg config.Configuration) (datastore.DatabaseClient, error) {
	dsn := cfg.Database.Dsn
	if util.IsStringEmpty(dsn) {
		dsn = "./convoy_db"
	}

	st, err := badgerhold.Open(badgerhold.Options{
		Encoder:          json.Marshal,
		Decoder:          json.Unmarshal,
		SequenceBandwith: 100,
		Options: badger.DefaultOptions(dsn).
			WithZSTDCompressionLevel(0).
			WithCompression(0).WithLogger(&logrus.Logger{Out: io.Discard}),
	})
	if err != nil {
		return nil, err
	}

	c := &Client{
		store:             st,
		groupRepo:         NewGroupRepo(st),
		eventRepo:         NewEventRepo(st),
		apiKeyRepo:        NewApiRoleRepo(st),
		applicationRepo:   NewApplicationRepo(st),
		eventDeliveryRepo: NewEventDeliveryRepository(st),
	}

	return c, nil
}

func (c *Client) Disconnect(context.Context) error {
	return c.store.Close()
}

func (c *Client) GetName() string {
	return "badger"
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
