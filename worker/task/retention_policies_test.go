package task

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	convoyMongo "github.com/frain-dev/convoy/datastore/mongo"

	"github.com/frain-dev/convoy/internal/pkg/searcher"
	noopsearcher "github.com/frain-dev/convoy/internal/pkg/searcher/noop"
	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/server/testdb"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RetentionPoliciesIntegrationTestSuite struct {
	suite.Suite
	DB        convoyMongo.Client
	ConvoyApp *applicationHandler
}

func (r *RetentionPoliciesIntegrationTestSuite) SetupSuite() {
	r.DB = getDB()
	r.ConvoyApp = buildApplication()
}

func (r *RetentionPoliciesIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(r.T(), r.DB)
}

func (r *RetentionPoliciesIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(r.T(), r.DB)
}

func (r *RetentionPoliciesIntegrationTestSuite) Test_Should_Export_Two_Documents() {
	// seed instance configuration
	_, err := seedConfiguration(r.ConvoyApp.store)
	require.NoError(r.T(), err)

	// seed Project
	projectConfig := &datastore.ProjectConfig{
		Signature: &datastore.SignatureConfiguration{
			Header: "X-Convoy-Signature",
			Versions: []datastore.SignatureVersion{
				{
					UID:       uuid.NewString(),
					Hash:      "SHA256",
					Encoding:  datastore.HexEncoding,
					CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
				},
			},
		},
		Strategy: &datastore.StrategyConfiguration{
			Type:       "linear",
			Duration:   20,
			RetryCount: 4,
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy: "72h",
		},
		RateLimit:                &datastore.DefaultRateLimitConfig,
		ReplayAttacks:            true,
		IsRetentionPolicyEnabled: true,
	}
	project, err := testdb.SeedProject(r.ConvoyApp.store, uuid.NewString(), uuid.NewString(), "test", datastore.OutgoingProject, projectConfig)

	require.NoError(r.T(), err)
	// seed event
	duration, err := time.ParseDuration("80h")
	require.NoError(r.T(), err)

	event, err := seedEvent(r.ConvoyApp.store, uuid.NewString(), project.UID, "", "*", []byte(`{}`), SeedFilter{
		CreatedAt: time.Now().UTC().Add(-duration),
	})
	require.NoError(r.T(), err)

	// seed eventdelivery
	eventDelivery, err := seedEventDelivery(r.ConvoyApp.store, event.UID, uuid.NewString(), project.UID, "", datastore.SuccessEventStatus, uuid.NewString(), SeedFilter{
		CreatedAt: time.Now().UTC().Add(-duration),
	})
	require.NoError(r.T(), err)

	// call handler
	task := asynq.NewTask("retention-policies", nil, asynq.Queue(string(convoy.ScheduleQueue)))

	fn := RententionPolicies(getConfig(), r.ConvoyApp.configRepo, r.ConvoyApp.projectRepo, r.ConvoyApp.eventRepo, r.ConvoyApp.eventDeliveryRepo, r.ConvoyApp.searcher)
	err = fn(context.Background(), task)
	require.NoError(r.T(), err)

	// check that event and eventdelivery repos are empty
	_, err = r.ConvoyApp.eventRepo.FindEventByID(context.Background(), event.UID)
	require.ErrorIs(r.T(), err, datastore.ErrEventNotFound)

	_, err = r.ConvoyApp.eventDeliveryRepo.FindEventDeliveryByID(context.Background(), eventDelivery.UID)
	require.ErrorIs(r.T(), err, datastore.ErrEventDeliveryNotFound)

	// check the number of retained events on projects
	p, err := r.ConvoyApp.projectRepo.FetchProjectByID(context.Background(), project.UID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), p.Metadata.RetainedEvents, 1)
}

func (r *RetentionPoliciesIntegrationTestSuite) Test_Should_Export_Zero_Documents() {
	// seed instance configuration
	_, err := seedConfiguration(r.ConvoyApp.store)
	require.NoError(r.T(), err)

	// seed project
	projectConfig := &datastore.ProjectConfig{
		Signature: &datastore.SignatureConfiguration{
			Header: "X-Convoy-Signature",
			Versions: []datastore.SignatureVersion{
				{
					UID:       uuid.NewString(),
					Hash:      "SHA256",
					Encoding:  datastore.HexEncoding,
					CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
				},
			},
		},
		Strategy: &datastore.StrategyConfiguration{
			Type:       "linear",
			Duration:   20,
			RetryCount: 4,
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy: "72h",
		},
		RateLimit:                &datastore.DefaultRateLimitConfig,
		ReplayAttacks:            true,
		IsRetentionPolicyEnabled: true,
	}
	project, err := testdb.SeedProject(r.ConvoyApp.store, uuid.NewString(), uuid.NewString(), "test", datastore.OutgoingProject, projectConfig)

	require.NoError(r.T(), err)
	// seed event
	event, err := seedEvent(r.ConvoyApp.store, uuid.NewString(), project.UID, "", "*", []byte(`{}`), SeedFilter{
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(r.T(), err)

	// seed eventdelivery
	eventDelivery, err := seedEventDelivery(r.ConvoyApp.store, event.UID, uuid.NewString(), project.UID, "", datastore.SuccessEventStatus, uuid.NewString(), SeedFilter{
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(r.T(), err)

	// call handler
	task := asynq.NewTask(string(convoy.TaskName("retention-policies")), nil, asynq.Queue(string(convoy.ScheduleQueue)))

	fn := RententionPolicies(getConfig(), r.ConvoyApp.configRepo, r.ConvoyApp.projectRepo, r.ConvoyApp.eventRepo, r.ConvoyApp.eventDeliveryRepo, r.ConvoyApp.searcher)
	err = fn(context.Background(), task)
	require.NoError(r.T(), err)

	// check that event and eventdelivery is not empty
	e, err := r.ConvoyApp.eventRepo.FindEventByID(context.Background(), event.UID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), e.UID, event.UID)

	ed, err := r.ConvoyApp.eventDeliveryRepo.FindEventDeliveryByID(context.Background(), eventDelivery.UID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), ed.UID, eventDelivery.UID)
}

func TestRetentionPoliciesIntegrationSuiteTest(t *testing.T) {
	suite.Run(t, new(RetentionPoliciesIntegrationTestSuite))
}

func getMongoDSN() string {
	return os.Getenv("TEST_MONGO_DSN")
}

func getConfig() config.Configuration {
	return config.Configuration{
		Database: config.DatabaseConfiguration{
			Type: config.MongodbDatabaseProvider,
			Dsn:  getMongoDSN(),
		},
	}
}

func getDB() convoyMongo.Client {
	db, err := convoyMongo.New(getConfig())
	if err != nil {
		panic(fmt.Sprintf("failed to connect to db: %v", err))
	}
	_ = os.Setenv("TZ", "") // Use UTC by default :)

	return *db
}

func buildApplication() *applicationHandler {
	db := getDB()
	searcher := noopsearcher.NewNoopSearcher()
	store := datastore.New(db.Database())

	projectRepo := convoyMongo.NewProjectRepo(store)
	eventRepo := convoyMongo.NewEventRepository(store)
	configRepo := convoyMongo.NewConfigRepo(store)
	eventDeliveryRepo := convoyMongo.NewEventDeliveryRepository(store)

	app := &applicationHandler{
		projectRepo:       projectRepo,
		eventRepo:         eventRepo,
		configRepo:        configRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		searcher:          searcher,
		store:             store,
	}

	return app
}

type applicationHandler struct {
	projectRepo       datastore.ProjectRepository
	eventRepo         datastore.EventRepository
	configRepo        datastore.ConfigurationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	searcher          searcher.Searcher
	store             datastore.Store
}

func seedEvent(store datastore.Store, endpointID string, projectID string, uid, eventType string, data []byte, filter SeedFilter) (*datastore.Event, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	ev := &datastore.Event{
		UID:       uid,
		EventType: datastore.EventType(eventType),
		Data:      data,
		Endpoints: []string{endpointID},
		ProjectID: projectID,
		CreatedAt: primitive.NewDateTimeFromTime(time.Unix(filter.CreatedAt.Unix(), 0)),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	// Seed Data.
	eventRepo := convoyMongo.NewEventRepository(store)
	err := eventRepo.CreateEvent(context.TODO(), ev)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

func seedEventDelivery(store datastore.Store, eventID string, endpointID string, projectID string, uid string, status datastore.EventDeliveryStatus, subcriptionID string, filter SeedFilter) (*datastore.EventDelivery, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	eventDelivery := &datastore.EventDelivery{
		UID:            uid,
		EventID:        eventID,
		EndpointID:     endpointID,
		Status:         status,
		SubscriptionID: subcriptionID,
		ProjectID:      projectID,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Unix(filter.CreatedAt.Unix(), 0)),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	// Seed Data.
	eventDeliveryRepo := convoyMongo.NewEventDeliveryRepository(store)
	err := eventDeliveryRepo.CreateEventDelivery(context.TODO(), eventDelivery)
	if err != nil {
		return nil, err
	}

	return eventDelivery, nil
}

func seedConfiguration(store datastore.Store) (*datastore.Configuration, error) {
	defaultStorage := &datastore.DefaultStoragePolicy
	defaultStorage.OnPrem.Path = "/tmp/convoy/export/"

	config := &datastore.Configuration{
		UID:                uuid.NewString(),
		IsAnalyticsEnabled: true,
		StoragePolicy:      defaultStorage,
	}

	// Seed Data
	configRepo := convoyMongo.NewConfigRepo(store)
	err := configRepo.CreateConfiguration(context.TODO(), config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

type SeedFilter struct {
	CreatedAt time.Time
}
