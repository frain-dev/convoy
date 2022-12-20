package mongo

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/newrelic/go-agent/v3/integrations/nrmongo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Client struct {
	db *mongo.Database
}

func New(cfg config.Configuration) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Client()
	newRelicMonitor := nrmongo.NewCommandMonitor(nil)
	opts.SetMonitor(newRelicMonitor)
	opts.ApplyURI(cfg.Database.Dsn)

	client, err := mongo.Connect(ctx, opts)
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
		db: conn,
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

func (c *Client) Database() *mongo.Database {
	return c.db
}

func (c *Client) ensureMongoIndices() {
	c.ensureIndex(datastore.ProjectsCollection, "uid", true, nil)
	c.ensureIndex(datastore.FilterCollection, "uid", true, nil)

	c.ensureIndex(datastore.OrganisationCollection, "uid", true, nil)

	c.ensureIndex(datastore.OrganisationMembersCollection, "organisation_id", false, nil)
	c.ensureIndex(datastore.OrganisationMembersCollection, "user_id", false, nil)
	c.ensureIndex(datastore.OrganisationMembersCollection, "uid", true, nil)

	c.ensureIndex(datastore.OrganisationInvitesCollection, "uid", true, nil)
	c.ensureIndex(datastore.OrganisationInvitesCollection, "token", true, nil)

	c.ensureIndex(datastore.EndpointCollection, "project_id", false, nil)
	c.ensureIndex(datastore.EndpointCollection, "uid", true, nil)
	c.ensureIndex(datastore.EndpointCollection, "owner_id", false, nil)

	c.ensureIndex(datastore.UserCollection, "uid", true, nil)

	c.ensureIndex(datastore.APIKeyCollection, "uid", true, nil)
	c.ensureIndex(datastore.APIKeyCollection, "mask_id", true, nil)

	c.ensureIndex(datastore.UserCollection, "uid", true, nil)

	c.ensureIndex(datastore.APIKeyCollection, "uid", true, nil)
	c.ensureIndex(datastore.APIKeyCollection, "mask_id", true, nil)

	c.ensureIndex(datastore.EventCollection, "uid", true, nil)
	c.ensureIndex(datastore.EventCollection, "endpoints", false, nil)
	c.ensureIndex(datastore.EventCollection, "project_id", false, nil)

	c.ensureIndex(datastore.EventDeliveryCollection, "group_id", false, nil)
	c.ensureIndex(datastore.EventDeliveryCollection, "status", false, nil)

	c.ensureIndex(datastore.SourceCollection, "uid", true, nil)
	c.ensureIndex(datastore.SourceCollection, "mask_id", true, nil)

	c.ensureIndex(datastore.SubscriptionCollection, "uid", true, nil)
	c.ensureIndex(datastore.SubscriptionCollection, "filter_config.event_types", false, nil)

	// register compound indexes
	c.ensureCompoundIndex(datastore.EndpointCollection)
	c.ensureCompoundIndex(datastore.UserCollection)
	c.ensureCompoundIndex(datastore.EventCollection)
	c.ensureCompoundIndex(datastore.ProjectsCollection)
	c.ensureCompoundIndex(datastore.DeviceCollection)
	c.ensureCompoundIndex(datastore.APIKeyCollection)
	c.ensureCompoundIndex(datastore.SubscriptionCollection)
	c.ensureCompoundIndex(datastore.EventDeliveryCollection)
	c.ensureCompoundIndex(datastore.OrganisationInvitesCollection)
	c.ensureCompoundIndex(datastore.OrganisationMembersCollection)
	c.ensureCompoundIndex(datastore.PortalLinkCollection)
}

// ensureIndex - ensures an index is created for a specific field in a collection
func (c *Client) ensureIndex(collectionName string, field string, unique bool, partialFilterExpression interface{}) bool {
	createIndexOpts := &options.IndexOptions{Unique: &unique}

	if partialFilterExpression != nil {
		createIndexOpts.SetPartialFilterExpression(partialFilterExpression)
	}

	mod := mongo.IndexModel{
		Keys:    bson.D{{Key: field, Value: 1}}, // index in ascending order or -1 for descending order
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

func (c *Client) ensureCompoundIndex(collectionName string) bool {
	collection := c.db.Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	compoundIndices := compoundIndices()

	compoundIndex, ok := compoundIndices[collectionName]

	if !ok {
		return false
	}

	_, err := collection.Indexes().CreateMany(ctx, compoundIndex)
	if err != nil {
		log.WithError(err).Errorf("failed to create index on collection %s", collectionName)
		return false
	}

	return true
}

func compoundIndices() map[string][]mongo.IndexModel {
	compoundIndices := map[string][]mongo.IndexModel{
		datastore.ProjectsCollection: {
			{
				Keys: bson.D{
					{Key: "organisation_id", Value: 1},
					{Key: "name", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},

		datastore.EventCollection: {
			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
					{Key: "deleted_at", Value: 1},
					{Key: "project_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
					{Key: "deleted_at", Value: 1},
					{Key: "source_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
					{Key: "deleted_at", Value: 1},
					{Key: "endpoints", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
					{Key: "deleted_at", Value: 1},
					{Key: "project_id", Value: 1},
					{Key: "endpoints", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
					{Key: "deleted_at", Value: 1},
					{Key: "project_id", Value: 1},
					{Key: "source_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
					{Key: "deleted_at", Value: 1},
					{Key: "endpoints", Value: 1},
					{Key: "source_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
					{Key: "deleted_at", Value: 1},
					{Key: "project_id", Value: 1},
					{Key: "source_id", Value: 1},
					{Key: "endpoints", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "project_id", Value: 1},
					{Key: "deleted_at", Value: 1},
					{Key: "created_at", Value: -1},
				},
			},

			{
				Keys: bson.D{
					{Key: "endpoints", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
				},
			},

			{
				Keys: bson.D{
					{Key: "deleted_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "deleted_at", Value: -1},
				},
			},
		},

		datastore.EventDeliveryCollection: {
			{
				Keys: bson.D{
					{Key: "event_id", Value: 1},
					{Key: "deleted_at", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "event_id", Value: 1},
					{Key: "deleted_at", Value: 1},
					{Key: "created_at", Value: 1},
					{Key: "status", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "deleted_at", Value: 1},
					{Key: "created_at", Value: 1},
					{Key: "project_id", Value: 1},
					{Key: "status", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "endpoint_id", Value: 1},
					{Key: "deleted_at", Value: 1},
					{Key: "created_at", Value: 1},
					{Key: "group_id", Value: 1},
					{Key: "status", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "uid", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "deleted_at", Value: 1},
					{Key: "created_at", Value: 1},
					{Key: "project_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "deleted_at", Value: 1},
					{Key: "created_at", Value: -1},
					{Key: "group_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: 1},
					{Key: "group_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "deleted_at", Value: 1},
					{Key: "created_at", Value: -1},
					{Key: "endpoint_id", Value: 1},
					{Key: "project_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: 1},
				},
			},
		},

		datastore.EndpointCollection: {
			{
				Keys: bson.D{
					{Key: "project_id", Value: 1},
					{Key: "deleted_at", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},
		},

		datastore.OrganisationInvitesCollection: {
			{
				Keys: bson.D{
					{Key: "organisation_id", Value: 1},
					{Key: "invitee_email", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
			{
				Keys: bson.D{
					{Key: "token", Value: 1},
					{Key: "email", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
			},
		},

		datastore.OrganisationMembersCollection: {
			{
				Keys: bson.D{
					{Key: "organisation_id", Value: 1},
					{Key: "user_id", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},

		datastore.UserCollection: {
			{
				Keys: bson.D{
					{Key: "email", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},

		datastore.DeviceCollection: {
			{
				Keys: bson.D{
					{Key: "endpoint_id", Value: 1},
					{Key: "project_id", Value: 1},
					{Key: "host_name", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},

		datastore.SubscriptionCollection: {
			{
				Keys: bson.D{
					{Key: "uid", Value: 1},
					{Key: "project_id", Value: 1},
					{Key: "source_id", Value: 1},
					{Key: "device_id", Value: 1},
					{Key: "endpoint_id", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},

		datastore.APIKeyCollection: {
			{
				Keys: bson.D{
					{Key: "hash", Value: 1},
					{Key: "user_id", Value: 1},
					{Key: "key_type", Value: 1},
					{Key: "role.app", Value: 1},
					{Key: "role.project", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
			{
				Keys: bson.D{
					{Key: "hash", Value: 1},
					{Key: "mask_id", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},

		datastore.PortalLinkCollection: {
			{
				Keys: bson.D{
					{Key: "token", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},

		datastore.OrganisationCollection: {
			{
				Keys: bson.D{
					{Key: "custom_domain", Value: 1},
					{Key: "assigned_domain", Value: 1},
					{Key: "deleted_at", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},
	}

	return compoundIndices
}
