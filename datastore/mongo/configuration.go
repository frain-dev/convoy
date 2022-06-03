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
}

func NewConfigRepo(db *mongo.Database) datastore.ConfigurationRepository {
	return &configRepo{
		innerDB: db,
		client:  db.Collection(ConfigCollection)}
}

func (c *configRepo) CreateConfiguration(ctx context.Context, config *datastore.Configuration) error {
	config.ID = primitive.NewObjectID()

	_, err := c.client.InsertOne(ctx, config)
	return err
}

func (c *configRepo) LoadConfiguration(ctx context.Context) (*datastore.Configuration, error) {
	config := &datastore.Configuration{}

	filter := bson.M{"document_status": datastore.ActiveDocumentStatus}

	err := c.client.FindOne(ctx, filter).Decode(&config)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return config, datastore.ErrConfigNotFound
	}

	return config, err
}

func (c *configRepo) UpdateConfiguration(ctx context.Context, config *datastore.Configuration) error {
	filter := bson.M{"uid": config.UID, "document_status": datastore.ActiveDocumentStatus}

	update := bson.D{
		primitive.E{Key: "$set", Value: bson.D{
			primitive.E{Key: "is_analytics_enabled", Value: config.IsAnalyticsEnabled},
			primitive.E{Key: "updated_at", Value: primitive.NewDateTimeFromTime(time.Now())},
		}},
	}

	_, err := c.client.UpdateOne(ctx, filter, update)
	return err
}
