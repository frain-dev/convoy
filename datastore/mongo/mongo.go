package mongo

import (
	"context"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

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
	apiKeyRepo        datastore.APIKeyRepository
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	applicationRepo   datastore.ApplicationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
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
		apiKeyRepo:        NewApiKeyRepo(conn),
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

func (c *Client) APIRepo() datastore.APIKeyRepository {
	return c.apiKeyRepo
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

func (c *Client) ensureMongoIndices() {
	c.ensureIndex(GroupCollection, "uid", true, nil)
	c.ensureIndex(GroupCollection, "name", true, bson.M{"document_status": bson.M{"$eq": datastore.ActiveDocumentStatus}})
	c.ensureIndex(AppCollections, "uid", true, nil)
	c.ensureIndex(EventCollection, "uid", true, nil)
	c.ensureIndex(EventCollection, "event_type", false, nil)
	c.ensureIndex(EventCollection, "app_metadata.uid", false, nil)
	c.ensureIndex(AppCollections, "group_id", false, nil)
}

// ensureIndex - ensures an index is created for a specific field in a collection
func (c *Client) ensureIndex(collectionName string, field string, unique bool, partialFilterExpression interface{}) bool {
	createIndexOpts := &options.IndexOptions{Unique: &unique}

	if partialFilterExpression != nil {
		createIndexOpts.SetPartialFilterExpression(partialFilterExpression)
	}

	mod := mongo.IndexModel{
		Keys:    bson.M{field: 1}, // index in ascending order or -1 for descending order
		Options: createIndexOpts,
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
