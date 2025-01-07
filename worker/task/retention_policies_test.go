package task

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"os"
	"testing"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/oklog/ulid/v2"

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
		SSL:           &datastore.DefaultSSLConfig,
		RateLimit:     &datastore.DefaultRateLimitConfig,
		ReplayAttacks: true,
		CircuitBreakerConfig: &datastore.CircuitBreakerConfig{
			SampleRate:                  2,
			ErrorTimeout:                30,
			FailureThreshold:            10,
			SuccessThreshold:            1,
			ObservabilityWindow:         5,
			ConsecutiveFailureThreshold: 10,
		},
	}
	project, err := testdb.SeedProject(r.ConvoyApp.database, ulid.Make().String(), "test", r.DefaultOrg.UID, datastore.OutgoingProject, projectConfig)
	require.NoError(r.T(), err)

	endpoint, err := testdb.SeedEndpoint(r.DB, project, ulid.Make().String(), "test-endpoint", "", false, datastore.ActiveEndpointStatus)
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

	attempt1, err := seedDeliveryAttempt(r.ConvoyApp.database, eventDelivery1, project, endpoint, SeedFilter{
		CreatedAt: now,
	})
	require.NoError(r.T(), err)

	attempt2, err := seedDeliveryAttempt(r.ConvoyApp.database, eventDelivery2, project, endpoint, SeedFilter{
		CreatedAt: now,
	})
	require.NoError(r.T(), err)

	// call handler
	task := asynq.NewTask("retention-policies", nil, asynq.Queue(string(convoy.ScheduleQueue)))

	fn := RetentionPolicies(r.DB, r.ConvoyApp.configRepo, r.ConvoyApp.projectRepo, r.ConvoyApp.eventRepo, r.ConvoyApp.eventDeliveryRepo, r.ConvoyApp.deliveryRepo, r.ConvoyApp.redis)
	err = fn(context.Background(), task)
	require.NoError(r.T(), err)

	_, err = r.ConvoyApp.deliveryRepo.FindDeliveryAttemptById(context.Background(), eventDelivery1.UID, attempt1.UID)
	require.ErrorIs(r.T(), err, datastore.ErrDeliveryAttemptNotFound)

	_, err = r.ConvoyApp.deliveryRepo.FindDeliveryAttemptById(context.Background(), eventDelivery2.UID, attempt2.UID)
	require.ErrorIs(r.T(), err, datastore.ErrDeliveryAttemptNotFound)

	// check that attempts, events and event delivery repos are empty
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
		SSL: &datastore.DefaultSSLConfig,
		Strategy: &datastore.StrategyConfiguration{
			Type:       "linear",
			Duration:   20,
			RetryCount: 4,
		},
		RateLimit:     &datastore.DefaultRateLimitConfig,
		ReplayAttacks: true,
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

	attempt, err := seedDeliveryAttempt(r.ConvoyApp.database, eventDelivery, project, endpoint, SeedFilter{
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(r.T(), err)

	// call handler
	task := asynq.NewTask(string(convoy.TaskName("retention-policies")), nil, asynq.Queue(string(convoy.ScheduleQueue)))

	fn := RetentionPolicies(r.DB, r.ConvoyApp.configRepo, r.ConvoyApp.projectRepo, r.ConvoyApp.eventRepo, r.ConvoyApp.eventDeliveryRepo, r.ConvoyApp.deliveryRepo, r.ConvoyApp.redis)
	err = fn(context.Background(), task)
	require.NoError(r.T(), err)

	a, err := r.ConvoyApp.deliveryRepo.FindDeliveryAttemptById(context.Background(), eventDelivery.UID, attempt.UID)
	require.NoError(r.T(), err)
	require.NotEqual(r.T(), a.CreatedAt, time.Now().UTC())
	require.NotEqual(r.T(), a.UpdatedAt, time.Now().UTC())

	// check that event and eventdelivery is not empty
	e, err := r.ConvoyApp.eventRepo.FindEventByID(context.Background(), project.UID, event.UID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), e.UID, event.UID)
	require.NotEqual(r.T(), e.AcknowledgedAt, time.Time{})
	require.NotEqual(r.T(), e.CreatedAt, time.Time{})
	require.NotEqual(r.T(), e.UpdatedAt, time.Time{})

	ed, err := r.ConvoyApp.eventDeliveryRepo.FindEventDeliveryByID(context.Background(), project.UID, eventDelivery.UID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), ed.UID, eventDelivery.UID)
	require.NotEqual(r.T(), ed.AcknowledgedAt, time.Time{})
	require.NotEqual(r.T(), ed.CreatedAt, time.Time{})
	require.NotEqual(r.T(), ed.UpdatedAt, time.Time{})
}

func TestRetentionPoliciesIntegrationSuiteTest(t *testing.T) {
	suite.Run(t, new(RetentionPoliciesIntegrationTestSuite))
}

func getConfig() config.Configuration {
	_ = os.Setenv("CONVOY_DB_HOST", os.Getenv("TEST_REDIS_HOST"))
	_ = os.Setenv("CONVOY_REDIS_SCHEME", os.Getenv("TEST_REDIS_SCHEME"))
	_ = os.Setenv("CONVOY_REDIS_PORT", os.Getenv("TEST_REDIS_PORT"))

	_ = os.Setenv("CONVOY_DB_HOST", os.Getenv("TEST_DB_HOST"))
	_ = os.Setenv("CONVOY_DB_SCHEME", os.Getenv("TEST_DB_SCHEME"))
	_ = os.Setenv("CONVOY_DB_USERNAME", os.Getenv("TEST_DB_USERNAME"))
	_ = os.Setenv("CONVOY_DB_PASSWORD", os.Getenv("TEST_DB_PASSWORD"))
	_ = os.Setenv("CONVOY_DB_DATABASE", os.Getenv("TEST_DB_DATABASE"))
	_ = os.Setenv("CONVOY_DB_OPTIONS", os.Getenv("TEST_DB_OPTIONS"))
	_ = os.Setenv("CONVOY_DB_PORT", os.Getenv("TEST_DB_PORT"))

	_ = os.Setenv("CONVOY_LOCAL_ENCRYPTION_KEY", "test-key")

	err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Get()
	if err != nil {
		log.Fatal(err)
	}

	km, err := keys.NewLocalKeyManager()
	if err != nil {
		log.Fatal(err)
	}
	if km.IsSet() {
		if _, err = km.GetCurrentKey(); err != nil {
			log.Fatal(err)
		}
	}
	if err = keys.Set(km); err != nil {
		log.Fatal(err)
	}

	return cfg
}

func getDB() database.Database {
	db, err := postgres.NewDB(getConfig())
	if err != nil {
		panic(fmt.Sprintf("failed to connect to db: %v", err))
	}
	_ = os.Setenv("TZ", "") // Use UTC by default :)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(data interface{}, changelog interface{}) {})

	return db
}

func buildApplication() *applicationHandler {
	db := getDB()
	redis, err := rdb.NewClient(getConfig().Redis.BuildDsn())
	if err != nil {
		log.Fatal(err)
	}

	projectRepo := postgres.NewProjectRepo(db)
	eventRepo := postgres.NewEventRepo(db)
	configRepo := postgres.NewConfigRepo(db)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
	deliveryRepo := postgres.NewDeliveryAttemptRepo(db)

	app := &applicationHandler{
		projectRepo:       projectRepo,
		eventRepo:         eventRepo,
		configRepo:        configRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		deliveryRepo:      deliveryRepo,
		database:          db,
		redis:             redis,
	}

	return app
}

type applicationHandler struct {
	projectRepo       datastore.ProjectRepository
	eventRepo         datastore.EventRepository
	configRepo        datastore.ConfigurationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	deliveryRepo      datastore.DeliveryAttemptsRepository
	database          database.Database
	redis             *rdb.Redis
}

func seedEvent(db database.Database, endpointID string, projectID string, uid, eventType string, data []byte, filter SeedFilter) (*datastore.Event, error) {
	if util.IsStringEmpty(uid) {
		uid = ulid.Make().String()
	}

	ev := &datastore.Event{
		UID:            uid,
		EventType:      datastore.EventType(eventType),
		Data:           data,
		Endpoints:      []string{endpointID},
		ProjectID:      projectID,
		AcknowledgedAt: null.TimeFrom(time.Unix(filter.AcknowledgedAt.Unix(), 0)),
		CreatedAt:      time.Unix(filter.CreatedAt.Unix(), 0),
	}

	// Seed Data.
	eventRepo := postgres.NewEventRepo(db)
	err := eventRepo.CreateEvent(context.TODO(), ev)
	if err != nil {
		return nil, err
	}

	ev1, err := eventRepo.FindEventByID(context.TODO(), projectID, uid)
	if err != nil {
		return nil, err
	}
	ev1.CreatedAt = time.Unix(filter.CreatedAt.Unix(), 0)
	_, err = db.GetDB().ExecContext(context.Background(), "UPDATE convoy.events SET created_at=$1 WHERE id=$2", ev1.CreatedAt, uid)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

func seedEventDelivery(db database.Database, eventID string, endpointID string, projectID string, uid string, status datastore.EventDeliveryStatus, subscriptionID string, filter SeedFilter) (*datastore.EventDelivery, error) {
	if util.IsStringEmpty(uid) {
		uid = ulid.Make().String()
	}

	eventDelivery := &datastore.EventDelivery{
		UID:            uid,
		EventID:        eventID,
		EndpointID:     endpointID,
		Status:         status,
		SubscriptionID: subscriptionID,
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
		CLIMetadata:    &datastore.CLIMetadata{},
		Description:    "test",
		AcknowledgedAt: null.TimeFrom(filter.AcknowledgedAt),
		CreatedAt:      filter.CreatedAt,
	}

	// Seed Data.
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
	err := eventDeliveryRepo.CreateEventDelivery(context.TODO(), eventDelivery)
	if err != nil {
		return nil, err
	}

	eventDelivery.CreatedAt = time.Unix(filter.CreatedAt.Unix(), 0)
	_, err = db.GetDB().ExecContext(context.Background(), "UPDATE convoy.event_deliveries SET created_at=$1 WHERE id=$2", eventDelivery.CreatedAt, uid)
	if err != nil {
		return nil, err
	}

	return eventDelivery, nil
}

func seedDeliveryAttempt(db database.Database, delivery *datastore.EventDelivery, project *datastore.Project, endpoint *datastore.Endpoint, filter SeedFilter) (*datastore.DeliveryAttempt, error) {
	deliveryAttempt := &datastore.DeliveryAttempt{
		UID:              ulid.Make().String(),
		EventDeliveryId:  delivery.UID,
		URL:              "127.0.0.1",
		Method:           "POST",
		EndpointID:       endpoint.UID,
		ProjectId:        project.UID,
		APIVersion:       "2024-01-01",
		IPAddress:        "117.0.0.1",
		RequestHeader:    map[string]string{"Content-Type": "application/json"},
		ResponseHeader:   map[string]string{"Content-Type": "application/json"},
		HttpResponseCode: "200",
		ResponseData:     []byte("200 OK"),
		Status:           true,
		CreatedAt:        filter.CreatedAt,
		UpdatedAt:        filter.CreatedAt,
	}

	daRepo := postgres.NewDeliveryAttemptRepo(db)
	err := daRepo.CreateDeliveryAttempt(context.TODO(), deliveryAttempt)
	if err != nil {
		return nil, err
	}

	deliveryAttempt.CreatedAt = time.Unix(filter.CreatedAt.Unix(), 0)
	_, err = db.GetDB().ExecContext(context.Background(), "UPDATE convoy.delivery_attempts SET created_at=$1 WHERE id=$2", deliveryAttempt.CreatedAt, deliveryAttempt.UID)
	if err != nil {
		return nil, err
	}

	return deliveryAttempt, nil
}

func seedConfiguration(db database.Database) (*datastore.Configuration, error) {
	defaultStorage := &datastore.DefaultStoragePolicy
	defaultStorage.OnPrem.Path = null.NewString("/tmp/convoy/export", true)

	c := &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: true,
		StoragePolicy:      defaultStorage,
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy:                   "72h",
			IsRetentionPolicyEnabled: true,
		},
		CircuitBreakerConfig: &datastore.CircuitBreakerConfig{
			SampleRate:                  2,
			ErrorTimeout:                30,
			FailureThreshold:            10,
			SuccessThreshold:            1,
			ObservabilityWindow:         5,
			ConsecutiveFailureThreshold: 10,
		},
	}

	// Seed Data
	configRepo := postgres.NewConfigRepo(db)
	err := configRepo.CreateConfiguration(context.TODO(), c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

type SeedFilter struct {
	AcknowledgedAt time.Time
	CreatedAt      time.Time
}
