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

	// seed group
	groupConfig := &datastore.GroupConfig{
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
		DisableEndpoint:          true,
		ReplayAttacks:            true,
		IsRetentionPolicyEnabled: true,
	}
	group, err := testdb.SeedGroup(r.ConvoyApp.store, uuid.NewString(), uuid.NewString(), "test", datastore.OutgoingGroup, groupConfig)

	require.NoError(r.T(), err)
	// seed event
	duration, err := time.ParseDuration("80h")
	require.NoError(r.T(), err)

	event, err := seedEvent(r.ConvoyApp.store, uuid.NewString(), group.UID, "", "*", []byte(`{}`), SeedFilter{
		CreatedAt:      time.Now().UTC().Add(-duration),
		DocumentStatus: datastore.ActiveDocumentStatus,
	})
	require.NoError(r.T(), err)

	//seed eventdelivery
	eventDelivery, err := seedEventDelivery(r.ConvoyApp.store, event.UID, uuid.NewString(), group.UID, "", datastore.SuccessEventStatus, uuid.NewString(), SeedFilter{
		CreatedAt:      time.Now().UTC().Add(-duration),
		DocumentStatus: datastore.ActiveDocumentStatus,
	})
	require.NoError(r.T(), err)

	// call handler
	task := asynq.NewTask(string(convoy.TaskName("retention-policies")), nil, asynq.Queue(string(convoy.ScheduleQueue)))

	fn := RententionPolicies(getConfig(), r.ConvoyApp.configRepo, r.ConvoyApp.groupRepo, r.ConvoyApp.eventRepo, r.ConvoyApp.eventDeliveryRepo, r.ConvoyApp.searcher)
	err = fn(context.Background(), task)
	require.NoError(r.T(), err)

	// check that event and eventdelivery repos are empty
	_, err = r.ConvoyApp.eventRepo.FindEventByID(context.Background(), event.UID)
	require.ErrorIs(r.T(), err, datastore.ErrEventNotFound)

	_, err = r.ConvoyApp.eventDeliveryRepo.FindEventDeliveryByID(context.Background(), eventDelivery.UID)
	require.ErrorIs(r.T(), err, datastore.ErrEventDeliveryNotFound)

	// check the number of retained events on groups
	g, err := r.ConvoyApp.groupRepo.FetchGroupByID(context.Background(), group.UID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), g.Metadata.RetainedEvents, 1)
}

func (r *RetentionPoliciesIntegrationTestSuite) Test_Should_Export_Zero_Documents() {
	// seed instance configuration
	_, err := seedConfiguration(r.ConvoyApp.store)
	require.NoError(r.T(), err)

	// seed group
	groupConfig := &datastore.GroupConfig{
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
		DisableEndpoint:          true,
		ReplayAttacks:            true,
		IsRetentionPolicyEnabled: true,
	}
	group, err := testdb.SeedGroup(r.ConvoyApp.store, uuid.NewString(), uuid.NewString(), "test", datastore.OutgoingGroup, groupConfig)

	require.NoError(r.T(), err)
	// seed event
	event, err := seedEvent(r.ConvoyApp.store, uuid.NewString(), group.UID, "", "*", []byte(`{}`), SeedFilter{
		CreatedAt:      time.Now().UTC(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	})
	require.NoError(r.T(), err)

	//seed eventdelivery
	eventDelivery, err := seedEventDelivery(r.ConvoyApp.store, event.UID, uuid.NewString(), group.UID, "", datastore.SuccessEventStatus, uuid.NewString(), SeedFilter{
		CreatedAt:      time.Now().UTC(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	})
	require.NoError(r.T(), err)

	// call handler
	task := asynq.NewTask(string(convoy.TaskName("retention-policies")), nil, asynq.Queue(string(convoy.ScheduleQueue)))

	fn := RententionPolicies(getConfig(), r.ConvoyApp.configRepo, r.ConvoyApp.groupRepo, r.ConvoyApp.eventRepo, r.ConvoyApp.eventDeliveryRepo, r.ConvoyApp.searcher)
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

	groupRepo := convoyMongo.NewGroupRepo(store)
	eventRepo := convoyMongo.NewEventRepository(store)
	configRepo := convoyMongo.NewConfigRepo(store)
	eventDeliveryRepo := convoyMongo.NewEventDeliveryRepository(store)

	app := &applicationHandler{
		groupRepo:         groupRepo,
		eventRepo:         eventRepo,
		configRepo:        configRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		searcher:          searcher,
		store:             store,
	}

	return app
}

type applicationHandler struct {
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	configRepo        datastore.ConfigurationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	searcher          searcher.Searcher
	store             datastore.Store
}

func seedEvent(store datastore.Store, endpointID string, groupID string, uid, eventType string, data []byte, filter SeedFilter) (*datastore.Event, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	ev := &datastore.Event{
		UID:            uid,
		EventType:      datastore.EventType(eventType),
		Data:           data,
		Endpoints:      []string{endpointID},
		GroupID:        groupID,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Unix(filter.CreatedAt.Unix(), 0)),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: filter.DocumentStatus,
	}

	// Seed Data.
	eventRepo := convoyMongo.NewEventRepository(store)
	err := eventRepo.CreateEvent(context.TODO(), ev)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

func seedEventDelivery(store datastore.Store, eventID string, endpointID string, groupID string, uid string, status datastore.EventDeliveryStatus, subcriptionID string, filter SeedFilter) (*datastore.EventDelivery, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	eventDelivery := &datastore.EventDelivery{
		UID:            uid,
		EventID:        eventID,
		EndpointID:     endpointID,
		Status:         status,
		SubscriptionID: subcriptionID,
		GroupID:        groupID,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Unix(filter.CreatedAt.Unix(), 0)),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: filter.DocumentStatus,
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
		DocumentStatus:     datastore.ActiveDocumentStatus,
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
	CreatedAt      time.Time
	DocumentStatus datastore.DocumentStatus
}
