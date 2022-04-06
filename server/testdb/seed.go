package testdb

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	mongoStore "github.com/frain-dev/convoy/datastore/mongo"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// SeedApplication is create random application for integration tests.
func SeedApplication(db *mongo.Database) (datastore.Application, error) {
	app := datastore.Application{
		ID:    primitive.NewObjectID(),
		Title: "Test Application",
	}

	// Seed Collection.
	client := db.Collection(mongoStore.AppCollections, nil)

	// Seed Data.
	_, err := client.InsertOne(context.Background(), app)
	if err != nil {
		return datastore.Application{}, err
	}

	return app, nil
}
