package testdb

import (
	"context"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	mongoStore "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// SeedApplication is create random application for integration tests.
func SeedApplication(db datastore.DatabaseClient, g *datastore.Group) (*datastore.Application, error) {
	app := &datastore.Application{
		UID:            uuid.New().String(),
		Title:          "Test Application",
		GroupID:        g.UID,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	// Seed Data.
	appRepo := db.AppRepo()
	err := appRepo.CreateApplication(context.TODO(), app)
	if err != nil {
		return &datastore.Application{}, err
	}

	return app, nil
}

func SeedEndpoint(db datastore.DatabaseClient, app *datastore.Application) (*datastore.Application, error) {
	endpoint := &datastore.Endpoint{
		UID:            uuid.New().String(),
		Status:         datastore.ActiveEndpointStatus,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	app.Endpoints = append(app.Endpoints, *endpoint)

	// Seed Data.
	appRepo := db.AppRepo()
	err := appRepo.UpdateApplication(context.TODO(), app)
	if err != nil {
		return &datastore.Application{}, err
	}

	return app, nil
}

// PurgeDB
func PurgeDB(db datastore.DatabaseClient) {
	client := db.Client().(*mongo.Database)
	appCollection := client.Collection(mongoStore.AppCollections, nil)
	appCollection.Drop(context.TODO())
}

// seed default group
func SeedDefaultGroup(db datastore.DatabaseClient) (*datastore.Group, error) {
	defaultGroup := &datastore.Group{
		UID:  uuid.New().String(),
		Name: "default-group",
		Config: &datastore.GroupConfig{
			Strategy: datastore.StrategyConfiguration{
				Type: config.DefaultStrategyProvider,
				Default: datastore.DefaultStrategyConfiguration{
					IntervalSeconds: 10,
					RetryLimit:      2,
				},
			},
			Signature: datastore.SignatureConfiguration{
				Header: config.DefaultSignatureHeader,
				Hash:   "SHA512",
			},
			DisableEndpoint: false,
			ReplayAttacks:   false,
		},
		RateLimit:         convoy.RATE_LIMIT,
		RateLimitDuration: convoy.RATE_LIMIT_DURATION,
		CreatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus:    datastore.ActiveDocumentStatus,
	}

	// Seed Data.
	groupRepo := db.GroupRepo()
	err := groupRepo.CreateGroup(context.TODO(), defaultGroup)
	if err != nil {
		return &datastore.Group{}, err
	}

	return defaultGroup, nil
}
