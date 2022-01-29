package datastore

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/frain-dev/convoy/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func New(cfg config.Configuration) (*mongo.Client, error) {
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

	return client, nil
}

// EnsureIndex - ensures an index is created for a specific field in a collection
func EnsureIndex(db *mongo.Database, collectionName string, field string, unique bool) bool {

	mod := mongo.IndexModel{
		Keys:    bson.M{field: 1}, // index in ascending order or -1 for descending order
		Options: options.Index().SetUnique(unique),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := db.Collection(collectionName)

	_, err := collection.Indexes().CreateOne(ctx, mod)
	if err != nil {
		log.WithError(err).Errorf("failed to create index on field %s in %s", field, collectionName)
		return false
	}

	return true
}

func EnsureCompoundIndex(db *mongo.Database, collectionName string) bool {
	collection := db.Collection(collectionName)

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
		EventCollection: {
			{
				Keys: bson.D{
					{Key: "app_metadata.group_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: -1},
				},
			},

			{
				Keys: bson.D{
					{Key: "app_metadata.group_id", Value: 1},
					{Key: "app_metadata.uid", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: -1},
				},
			},

			{
				Keys: bson.D{
					{Key: "app_metadata.uid", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: -1},
				},
			},

			{
				Keys: bson.D{
					{Key: "app_metadata.group_id", Value: 1},
					{Key: "app_metadata.uid", Value: 1},
					{Key: "created_at", Value: -1},
				},
			},

			{
				Keys: bson.D{
					{Key: "app_metadata.uid", Value: 1},
					{Key: "app_metadata.group_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "app_metadata.uid", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "app_metadata.group_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
				},
			},
		},

		EventDeliveryCollection: {
			{
				Keys: bson.D{
					{Key: "event_metadata.uid", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: -1},
					{Key: "app_metadata.group_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: -1},
					{Key: "app_metadata.uid", Value: 1},
					{Key: "app_metadata.group_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
				},
			},
		},

		AppCollections: {
			{
				Keys: bson.D{
					{Key: "group_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},
		},
	}

	return compoundIndices
}
