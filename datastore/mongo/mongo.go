package mongo

import (
	"context"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	GroupCollection = "groups"
	AppCollections  = "applications"
	EventCollection = "events"
)

type Client struct {
	db                *mongo.Database
	groupRepo         convoy.GroupRepository
	eventRepo         convoy.EventRepository
	applicationRepo   convoy.ApplicationRepository
	eventDeliveryRepo convoy.EventDeliveryRepository
}

func New(cfg config.Configuration) (datastore.DatabaseClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.Database.Dsn))
	if err != nil {
		return nil, err
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	u, err := url.Parse(cfg.Database.Dsn)
	if err != nil {
		return nil, err
	}

	dbName := strings.TrimPrefix(u.Path, "/")
	conn := client.Database(dbName, nil)

	c := &Client{
		db:                conn,
		groupRepo:         NewGroupRepo(conn),
		applicationRepo:   NewApplicationRepo(conn),
		eventRepo:         NewEventRepository(conn),
		eventDeliveryRepo: NewEventDeliveryRepository(conn),
	}

	c.ensureMongoIndices()

	return c, nil
}

func (c *Client) Disconnect(ctx context.Context) error {
	return c.db.Client().Disconnect(ctx)
}

func (c *Client) GetName() string {
	return "mongo"
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

func (c *Client) ensureMongoIndices() {
	c.ensureIndex(GroupCollection, "uid", true)
	c.ensureIndex(GroupCollection, "name", true)
	c.ensureIndex(AppCollections, "uid", true)
	c.ensureIndex(EventCollection, "uid", true)
	c.ensureIndex(EventCollection, "event_type", false)
}

// ensureIndex - ensures an index is created for a specific field in a collection
func (c *Client) ensureIndex(collectionName string, field string, unique bool) bool {

	mod := mongo.IndexModel{
		Keys:    bson.M{field: 1}, // index in ascending order or -1 for descending order
		Options: options.Index().SetUnique(unique),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := c.db.Collection(collectionName)

	_, err := collection.Indexes().CreateOne(ctx, mod)
	if err != nil {
		log.WithError(err).Errorf("failed to create index on field %s in %s", field, collectionName)
		return false
	}

	return true
}
