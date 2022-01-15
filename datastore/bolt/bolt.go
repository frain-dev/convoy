package bolt

import (
	"context"

	"go.etcd.io/bbolt"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

type Client struct {
	db                *bbolt.DB
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	applicationRepo   datastore.ApplicationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
}

const bucketName string = "convoy"

func New(cfg config.Configuration) (datastore.DatabaseClient, error) {
	db, err := bbolt.Open("datastore.db", 0666, nil)
	if err != nil {
		return nil, err
	}

	bErr := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}

		_, err = tx.CreateBucketIfNotExists([]byte(eventDeliveryBucketName))
		if err != nil {
			return err
		}

		return err
	})

	if bErr != nil {
		return nil, bErr
	}

	c := &Client{
		db:        db,
		groupRepo: NewGroupRepo(db),
		// applicationRepo:   NewApplicationRepo(conn),
		// eventRepo:         NewEventRepository(conn),
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
