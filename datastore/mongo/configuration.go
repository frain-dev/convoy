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
	store datastore.Store
}

func NewConfigRepo(store datastore.Store) datastore.ConfigurationRepository {
	return &configRepo{
		store: store,
	}
}

func (c *configRepo) CreateConfiguration(ctx context.Context, config *datastore.Configuration) error {
	ctx = c.setCollectionInContext(ctx)
	// config.ID = primitive.NewObjectID()

	return c.store.Save(ctx, config, nil)
}

func (c *configRepo) LoadConfiguration(ctx context.Context) (*datastore.Configuration, error) {
	ctx = c.setCollectionInContext(ctx)

	config := &datastore.Configuration{}

	filter := bson.M{}

	err := c.store.FindOne(ctx, filter, nil, config)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, datastore.ErrConfigNotFound
	}

	return config, err
}

func (c *configRepo) UpdateConfiguration(ctx context.Context, config *datastore.Configuration) error {
	ctx = c.setCollectionInContext(ctx)

	filter := bson.M{"uid": config.UID}

	update := bson.M{
		"$set": bson.M{
			"is_analytics_enabled": config.IsAnalyticsEnabled,
			"is_signup_enabled":    config.IsSignupEnabled,
			"storage_policy":       config.StoragePolicy,
			"updated_at":           primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	err := c.store.UpdateOne(ctx, filter, update)
	return err
}

func (db *configRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.ConfigCollection)
}
