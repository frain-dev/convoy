package testdb

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
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

func SeedEndpoint(db datastore.DatabaseClient, app *datastore.Application, events []string) (*datastore.Endpoint, error) {
	endpoint := &datastore.Endpoint{
		UID:            uuid.New().String(),
		Events:         events,
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

func SeedMultipleEndpoints(db datastore.DatabaseClient, app *datastore.Application, events []string, count int) ([]datastore.Endpoint, error) {
	for i := 0; i < count; i++ {
		endpoint := &datastore.Endpoint{
			UID:            uuid.New().String(),
			Events:         events,
			Status:         datastore.ActiveEndpointStatus,
			DocumentStatus: datastore.ActiveDocumentStatus,
		}

		app.Endpoints = append(app.Endpoints, *endpoint)
	}

	// Seed Data.
	appRepo := db.AppRepo()
	err := appRepo.UpdateApplication(context.TODO(), app)
	if err != nil {
		return nil, err
	}

	return app.Endpoints, nil
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

// SeedAPIKey creates random api key for integration tests.
func SeedAPIKey(db datastore.DatabaseClient, g *datastore.Group, uid, name, keyType string) (*datastore.APIKey, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	apiKey := &datastore.APIKey{
		UID:    uid,
		MaskID: fmt.Sprintf("mask-%s", uuid.NewString()),
		Name:   name,
		Type:   datastore.KeyType(keyType),
		Role: auth.Role{
			Type:   auth.RoleUIAdmin,
			Groups: []string{g.UID},
			Apps:   nil,
		},
		Hash:           fmt.Sprintf("hash-%s", uuid.NewString()),
		Salt:           fmt.Sprintf("salt-%s", uuid.NewString()),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err := db.APIRepo().CreateAPIKey(context.Background(), apiKey)
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

// SeedEvent creates a random event for integration tests.
func SeedEvent(db datastore.DatabaseClient, app *datastore.Application, uid, eventType string, data []byte) (*datastore.Event, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	ev := &datastore.Event{
		UID:       uid,
		EventType: datastore.EventType(eventType),
		Data:      data,
		AppMetadata: &datastore.AppMetadata{
			UID:          app.UID,
			Title:        app.Title,
			GroupID:      app.GroupID,
			SupportEmail: app.SupportEmail,
		},
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	// Seed Data.
	err := db.EventRepo().CreateEvent(context.TODO(), ev)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

// SeedEventDelivery creates a random event delivery for integration tests.
func SeedEventDelivery(db datastore.DatabaseClient, app *datastore.Application, event *datastore.Event, endpoint *datastore.Endpoint, uid string, status datastore.EventDeliveryStatus) (*datastore.EventDelivery, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	eventDelivery := &datastore.EventDelivery{
		UID: uid,
		EventMetadata: &datastore.EventMetadata{
			UID:       event.UID,
			EventType: event.EventType,
		},
		EndpointMetadata: &datastore.EndpointMetadata{
			UID:               endpoint.UID,
			TargetURL:         endpoint.TargetURL,
			Status:            endpoint.Status,
			Secret:            endpoint.Secret,
			HttpTimeout:       endpoint.HttpTimeout,
			RateLimit:         endpoint.RateLimit,
			RateLimitDuration: endpoint.RateLimitDuration,
			Sent:              false,
		},
		Status: status,
		AppMetadata: &datastore.AppMetadata{
			UID:          app.UID,
			Title:        app.Title,
			GroupID:      app.GroupID,
			SupportEmail: app.SupportEmail,
		},
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	// Seed Data.
	err := db.EventDeliveryRepo().CreateEventDelivery(context.TODO(), eventDelivery)
	if err != nil {
		return nil, err
	}

	return eventDelivery, nil
}

// PurgeDB is run after every test run and it's used to truncate the DB to have
// a clean slate in the next run.
func PurgeDB(db datastore.DatabaseClient) {
	client := db.Client().(*mongo.Database)
	err := client.Drop(context.TODO())
	if err != nil {
		log.WithError(err).Fatal("failed to truncate db")
	}
}
