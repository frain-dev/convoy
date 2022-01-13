package bolt

import (
	"context"

	"go.etcd.io/bbolt"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

type Client struct {
	db                *bbolt.DB
	groupRepo         convoy.GroupRepository
	eventRepo         convoy.EventRepository
	applicationRepo   convoy.ApplicationRepository
	eventDeliveryRepo convoy.EventDeliveryRepository
}

func New(cfg config.Configuration) (datastore.DatabaseClient, error) {
	db, err := bbolt.Open("convoy.db", 0666, nil)
	if err != nil {
		return nil, err
	}

	c := &Client{
		db:        db,
		groupRepo: NewGroupRepo(db),
		// applicationRepo:   NewApplicationRepo(conn),
		// eventRepo:         NewEventRepository(conn),
		// eventDeliveryRepo: NewEventDeliveryRepository(conn),
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

func (c *Client) GroupRepo() convoy.GroupRepository {
	return c.groupRepo
}

func (c *Client) AppRepo() convoy.ApplicationRepository {
	return c.applicationRepo
}

func (c *Client) EventRepo() convoy.EventRepository {
	return c.eventRepo
}

func (c *Client) EventDeliveryRepo() convoy.EventDeliveryRepository {
	return c.eventDeliveryRepo
}
