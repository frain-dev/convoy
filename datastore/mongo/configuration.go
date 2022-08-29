package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type configRepo struct {
	innerDB *mongo.Database
	client  *mongo.Collection
	store   datastore.Store
}

func NewConfigRepo(db *mongo.Database, store datastore.Store) datastore.ConfigurationRepository {
	return &configRepo{
		innerDB: db,
		client:  db.Collection(ConfigCollection),
		store:   store,
	}
}

func (c *configRepo) CreateConfiguration(ctx context.Context, config *datastore.Configuration) error {
	config.ID = primitive.NewObjectID()

	err := c.store.Save(ctx, config, nil)
	return err
}

func (c *configRepo) LoadConfiguration(ctx context.Context) (*datastore.Configuration, error) {
	config := &datastore.Configuration{}

	filter := bson.M{}

	err := c.store.FindOne(ctx, filter, nil, config)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, datastore.ErrConfigNotFound
	}

	return config, err
}

func (c *configRepo) UpdateConfiguration(ctx context.Context, config *datastore.Configuration) error {
	filter := bson.M{"uid": config.UID}

	update := bson.D{
		primitive.E{Key: "is_analytics_enabled", Value: config.IsAnalyticsEnabled},
		primitive.E{Key: "is_signup_enabled", Value: config.IsSignupEnabled},
		primitive.E{Key: "storage_policy", Value: config.StoragePolicy},
		primitive.E{Key: "updated_at", Value: primitive.NewDateTimeFromTime(time.Now())},
	}

	err := c.store.UpdateOne(ctx, filter, update)
	return err
}
