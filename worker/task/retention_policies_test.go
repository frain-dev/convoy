package task

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/internal/pkg/searcher"
	noopsearcher "github.com/frain-dev/convoy/internal/pkg/searcher/noop"
	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/util"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RetentionPoliciesIntegrationTestSuite struct {
	suite.Suite
	DB          database.Database
	ConvoyApp   *applicationHandler
	DefaultUser *datastore.User
	DefaultOrg  *datastore.Organisation
}

func (r *RetentionPoliciesIntegrationTestSuite) SetupSuite() {
	r.DB = getDB()
	r.ConvoyApp = buildApplication()
}

func (r *RetentionPoliciesIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(r.T(), r.DB)

	user, err := testdb.SeedDefaultUser(r.DB)
	require.NoError(r.T(), err)
	r.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(r.DB, user)
	require.NoError(r.T(), err)
	r.DefaultOrg = org
}

func (r *RetentionPoliciesIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(r.T(), r.DB)
}

func (r *RetentionPoliciesIntegrationTestSuite) Test_Should_Export_Two_Documents() {
	// seed instance configuration
	_, err := seedConfiguration(r.ConvoyApp.database)
	require.NoError(r.T(), err)

	// seed Project
	projectConfig := &datastore.ProjectConfig{
		Signature: &datastore.SignatureConfiguration{
			Header: "X-Convoy-Signature",
			Versions: []datastore.SignatureVersion{
				{
					UID:       ulid.Make().String(),
					Hash:      "SHA256",
					Encoding:  datastore.HexEncoding,
					CreatedAt: time.Now(),
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
	project, err := testdb.SeedProject(r.ConvoyApp.database, ulid.Make().String(), "test", r.DefaultOrg.UID, datastore.OutgoingProject, projectConfig)
	require.NoError(r.T(), err)

	endpoint, err := testdb.SeedEndpoint(r.DB, project, ulid.Make().String(), "test-endpoint", "", false, datastore.ActiveEndpointStatus)
	require.NoError(r.T(), err)

	require.NoError(r.T(), err)
	// seed event
	duration, err := time.ParseDuration("80h")
	require.NoError(r.T(), err)

	event1, err := seedEvent(r.ConvoyApp.database, endpoint.UID, project.UID, "", "*", []byte(`{}`), SeedFilter{
		CreatedAt: time.Now().UTC().Add(-duration),
	})
	require.NoError(r.T(), err)

	event2, err := seedEvent(r.ConvoyApp.database, endpoint.UID, project.UID, "", "*", []byte(`{}`), SeedFilter{
		CreatedAt: time.Now().UTC().Add(-duration),
	})
	require.NoError(r.T(), err)

	subscription, err := testdb.SeedSubscription(r.DB, project, "", project.Type, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(r.T(), err)

	now := time.Now().UTC().Add(-duration)
	// seed eventdelivery
	eventDelivery1, err := seedEventDelivery(r.ConvoyApp.database, event1.UID, endpoint.UID, project.UID, "", datastore.SuccessEventStatus, subscription.UID, SeedFilter{
		CreatedAt: now,
	})
	require.NoError(r.T(), err)

	eventDelivery2, err := seedEventDelivery(r.ConvoyApp.database, event2.UID, endpoint.UID, project.UID, "", datastore.SuccessEventStatus, subscription.UID, SeedFilter{
		CreatedAt: now,
	})
	require.NoError(r.T(), err)

	// call handler
	task := asynq.NewTask("retention-policies", nil, asynq.Queue(string(convoy.ScheduleQueue)))

	fn := RetentionPolicies(r.ConvoyApp.configRepo, r.ConvoyApp.projectRepo, r.ConvoyApp.eventRepo, r.ConvoyApp.eventDeliveryRepo, r.ConvoyApp.exportRepo, r.ConvoyApp.searcher)
	err = fn(context.Background(), task)
	require.NoError(r.T(), err)

	// check that event and eventdelivery repos are empty
	_, err = r.ConvoyApp.eventRepo.FindEventByID(context.Background(), project.UID, event1.UID)
	require.ErrorIs(r.T(), err, datastore.ErrEventNotFound)

	_, err = r.ConvoyApp.eventDeliveryRepo.FindEventDeliveryByID(context.Background(), project.UID, eventDelivery1.UID)
	require.ErrorIs(r.T(), err, datastore.ErrEventDeliveryNotFound)

	_, err = r.ConvoyApp.eventDeliveryRepo.FindEventDeliveryByID(context.Background(), project.UID, eventDelivery2.UID)
	require.ErrorIs(r.T(), err, datastore.ErrEventDeliveryNotFound)

	// check the number of retained events on projects
	p, err := r.ConvoyApp.projectRepo.FetchProjectByID(context.Background(), project.UID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), 2, p.RetainedEvents)
}

func (r *RetentionPoliciesIntegrationTestSuite) Test_Should_Export_Zero_Documents() {
	// seed instance configuration
	_, err := seedConfiguration(r.ConvoyApp.database)
	require.NoError(r.T(), err)

	// seed project
	projectConfig := &datastore.ProjectConfig{
		Signature: &datastore.SignatureConfiguration{
			Header: "X-Convoy-Signature",
			Versions: []datastore.SignatureVersion{
				{
					UID:       ulid.Make().String(),
					Hash:      "SHA256",
					Encoding:  datastore.HexEncoding,
					CreatedAt: time.Now(),
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
	project, err := testdb.SeedProject(r.ConvoyApp.database, ulid.Make().String(), "test", r.DefaultOrg.UID, datastore.OutgoingProject, projectConfig)
	require.NoError(r.T(), err)

	endpoint, err := testdb.SeedEndpoint(r.DB, project, ulid.Make().String(), "test-endpoint", "", false, datastore.ActiveEndpointStatus)
	require.NoError(r.T(), err)

	// seed event
	event, err := seedEvent(r.ConvoyApp.database, endpoint.UID, project.UID, "", "*", []byte(`{}`), SeedFilter{
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(r.T(), err)

	subscription, err := testdb.SeedSubscription(r.DB, project, "", project.Type, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(r.T(), err)

	// seed eventdelivery
	eventDelivery, err := seedEventDelivery(r.ConvoyApp.database, event.UID, endpoint.UID, project.UID, "", datastore.SuccessEventStatus, subscription.UID, SeedFilter{
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(r.T(), err)

	// call handler
	task := asynq.NewTask(string(convoy.TaskName("retention-policies")), nil, asynq.Queue(string(convoy.ScheduleQueue)))

	fn := RetentionPolicies(r.ConvoyApp.configRepo, r.ConvoyApp.projectRepo, r.ConvoyApp.eventRepo, r.ConvoyApp.eventDeliveryRepo, r.ConvoyApp.exportRepo, r.ConvoyApp.searcher)
	err = fn(context.Background(), task)
	require.NoError(r.T(), err)

	// check that event and eventdelivery is not empty
	e, err := r.ConvoyApp.eventRepo.FindEventByID(context.Background(), project.UID, event.UID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), e.UID, event.UID)

	ed, err := r.ConvoyApp.eventDeliveryRepo.FindEventDeliveryByID(context.Background(), project.UID, eventDelivery.UID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), ed.UID, eventDelivery.UID)
}

func TestRetentionPoliciesIntegrationSuiteTest(t *testing.T) {
	suite.Run(t, new(RetentionPoliciesIntegrationTestSuite))
}

func getConfig() config.Configuration {
	return config.Configuration{
		Database: config.DatabaseConfiguration{
			Type:     config.PostgresDatabaseProvider,
			Host:     os.Getenv("TEST_DB_HOST"),
			Scheme:   os.Getenv("TEST_DB_SCHEME"),
			Username: os.Getenv("TEST_DB_USERNAME"),
			Password: os.Getenv("TEST_DB_PASSWORD"),
			Database: os.Getenv("TEST_DB_DATABASE"),
			Port:     6379,
		},
	}
}

func getDB() database.Database {
	db, err := postgres.NewDB(getConfig())
	if err != nil {
		panic(fmt.Sprintf("failed to connect to db: %v", err))
	}
	_ = os.Setenv("TZ", "") // Use UTC by default :)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(data interface{}) {})

	return db
}

func buildApplication() *applicationHandler {
	db := getDB()
	searcher := noopsearcher.NewNoopSearcher()

	projectRepo := postgres.NewProjectRepo(db)
	eventRepo := postgres.NewEventRepo(db)
	configRepo := postgres.NewConfigRepo(db)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
	exportRepo := postgres.NewExportRepo(db)

	app := &applicationHandler{
		projectRepo:       projectRepo,
		eventRepo:         eventRepo,
		configRepo:        configRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		searcher:          searcher,
		database:          db,
		exportRepo:        exportRepo,
	}

	return app
}

type applicationHandler struct {
	projectRepo       datastore.ProjectRepository
	eventRepo         datastore.EventRepository
	configRepo        datastore.ConfigurationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	exportRepo        datastore.ExportRepository
	searcher          searcher.Searcher
	database          database.Database
}

func seedEvent(db database.Database, endpointID string, projectID string, uid, eventType string, data []byte, filter SeedFilter) (*datastore.Event, error) {
	if util.IsStringEmpty(uid) {
		uid = ulid.Make().String()
	}

	ev := &datastore.Event{
		UID:       uid,
		EventType: datastore.EventType(eventType),
		Data:      data,
		Endpoints: []string{endpointID},
		ProjectID: projectID,
		CreatedAt: time.Unix(filter.CreatedAt.Unix(), 0),
		UpdatedAt: time.Now(),
	}

	// Seed Data.
	eventRepo := postgres.NewEventRepo(db)
	err := eventRepo.CreateEvent(context.TODO(), ev)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

func seedEventDelivery(db database.Database, eventID string, endpointID string, projectID string, uid string, status datastore.EventDeliveryStatus, subcriptionID string, filter SeedFilter) (*datastore.EventDelivery, error) {
	if util.IsStringEmpty(uid) {
		uid = ulid.Make().String()
	}

	eventDelivery := &datastore.EventDelivery{
		UID:            uid,
		EventID:        eventID,
		EndpointID:     endpointID,
		Status:         status,
		SubscriptionID: subcriptionID,
		ProjectID:      projectID,
		Headers:        httpheader.HTTPHeader{"X-sig": []string{"3787 fmmfbf"}},
		DeliveryAttempts: []datastore.DeliveryAttempt{
			{UID: ulid.Make().String()},
		},
		Metadata: &datastore.Metadata{
			Data:            []byte(`{"name": "10x"}`),
			Raw:             `{"name": "10x"}`,
			Strategy:        datastore.ExponentialStrategyProvider,
			NextSendTime:    time.Now().Add(time.Hour),
			NumTrials:       1,
			IntervalSeconds: 10,
			RetryLimit:      20,
		},
		CLIMetadata: &datastore.CLIMetadata{},
		Description: "test",
		CreatedAt:   filter.CreatedAt,
		UpdatedAt:   time.Now(),
	}

	// Seed Data.
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
	err := eventDeliveryRepo.CreateEventDelivery(context.TODO(), eventDelivery)
	if err != nil {
		return nil, err
	}

	return eventDelivery, nil
}

func seedConfiguration(db database.Database) (*datastore.Configuration, error) {
	defaultStorage := &datastore.DefaultStoragePolicy
	defaultStorage.OnPrem.Path = null.NewString("/tmp/convoy/export/", true)

	config := &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: true,
		StoragePolicy:      defaultStorage,
	}

	// Seed Data
	configRepo := postgres.NewConfigRepo(db)
	err := configRepo.CreateConfiguration(context.TODO(), config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

type SeedFilter struct {
	CreatedAt time.Time
}
