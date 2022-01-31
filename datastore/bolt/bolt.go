package bolt

import (
	"context"

	"go.etcd.io/bbolt"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

type Client struct {
	db                *bbolt.DB
	apiKeyRepo        datastore.APIKeyRepository
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	applicationRepo   datastore.ApplicationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
}

func New(cfg config.Configuration) (datastore.DatabaseClient, error) {
	db, err := bbolt.Open(cfg.Database.Dsn, 0666, nil)
	if err != nil {
		return nil, err
	}

	c := &Client{
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
