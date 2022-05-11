package testdb

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	mongoStore "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// SeedApplication is create random application for integration tests.
func SeedApplication(db datastore.DatabaseClient, g *datastore.Group, uid, title string, disabled bool) (*datastore.Application, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	if util.IsStringEmpty(title) {
		title = fmt.Sprintf("TestApp-%s", uid)
	}

	app := &datastore.Application{
		UID:            uid,
		Title:          title,
		GroupID:        g.UID,
		IsDisabled:     disabled,
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

func SeedMultipleApplications(db datastore.DatabaseClient, g *datastore.Group, count int) error {
	for i := 0; i < count; i++ {
		uid := uuid.New().String()
		app := &datastore.Application{
			UID:            uid,
			Title:          fmt.Sprintf("Test-%s", uid),
			GroupID:        g.UID,
			IsDisabled:     false,
			DocumentStatus: datastore.ActiveDocumentStatus,
		}

		// Seed Data.
		appRepo := db.AppRepo()
		err := appRepo.CreateApplication(context.TODO(), app)
		if err != nil {
			return err
		}
	}
	return nil
}

func SeedEndpoint(db datastore.DatabaseClient, app *datastore.Application) (*datastore.Endpoint, error) {
	endpoint := &datastore.Endpoint{
		UID:            uuid.New().String(),
		Events:         []string{"*"},
		Status:         datastore.ActiveEndpointStatus,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	app.Endpoints = append(app.Endpoints, *endpoint)

	// Seed Data.
	appRepo := db.AppRepo()
	err := appRepo.UpdateApplication(context.TODO(), app)
	if err != nil {
		return &datastore.Endpoint{}, err
	}

	return endpoint, nil
}

func SeedMultipleEndpoints(db datastore.DatabaseClient, app *datastore.Application, count int) (*datastore.Application, error) {
	for i := 0; i < count; i++ {
		endpoint := &datastore.Endpoint{
			UID:            uuid.New().String(),
			Events:         []string{"*"},
			Status:         datastore.ActiveEndpointStatus,
			DocumentStatus: datastore.ActiveDocumentStatus,
		}

		app.Endpoints = append(app.Endpoints, *endpoint)
	}

	// Seed Data.
	appRepo := db.AppRepo()
	err := appRepo.UpdateApplication(context.TODO(), app)
	if err != nil {
		return &datastore.Application{}, err
	}

	return app, nil
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

// PurgeDB is run after every test run and it's used to truncate the DB to have
// a clean slate in the next run.
func PurgeDB(db datastore.DatabaseClient) {
	client := db.Client().(*mongo.Database)
	appCollection := client.Collection(mongoStore.AppCollections, nil)
	_ = appCollection.Drop(context.TODO())

	groupCollection := client.Collection(mongoStore.GroupCollection, nil)
	_ = groupCollection.Drop(context.TODO())

	eventCollection := client.Collection(mongoStore.EventCollection, nil)
	_ = eventCollection.Drop(context.TODO())

	eventDeliveryCollection := client.Collection(mongoStore.EventDeliveryCollection, nil)
	_ = eventDeliveryCollection.Drop(context.TODO())
}
