//go:build integration
// +build integration

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
	mongoStore "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/searcher"
	noopsearcher "github.com/frain-dev/convoy/searcher/noop"
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
	DB        datastore.DatabaseClient
	ConvoyApp *applicationHandler
}

func (r *RetentionPoliciesIntegrationTestSuite) SetupSuite() {
	r.DB = getDB()
	r.ConvoyApp = buildApplication()
}

func (r *RetentionPoliciesIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(r.DB)
}

func (r *RetentionPoliciesIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(r.DB)
}

func (r *RetentionPoliciesIntegrationTestSuite) Test_Should_Export_Two_Documents() {
	//seed instance configuration
	_, err := seedConfiguration(r.DB)
	require.NoError(r.T(), err)

	//seed group
	groupConfig := &datastore.GroupConfig{
		Signature: &datastore.SignatureConfiguration{
			Header: "X-Convoy-Signature",
			Hash:   "SHA256",
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
	group, err := testdb.SeedGroup(r.DB, uuid.NewString(), uuid.NewString(), "test", datastore.OutgoingGroup, groupConfig)

	require.NoError(r.T(), err)
	//seed event
	duration, err := time.ParseDuration("80h")
	require.NoError(r.T(), err)

	event, err := seedEvent(r.DB, uuid.NewString(), group.UID, "", "*", []byte(`{}`), SeedFilter{
		CreatedAt:      time.Now().UTC().Add(-duration),
		DocumentStatus: datastore.ActiveDocumentStatus,
	})
	require.NoError(r.T(), err)

	//seed eventdelivery
	eventDelivery, err := seedEventDelivery(r.DB, uuid.NewString(), event.UID, uuid.NewString(), group.UID, "", datastore.SuccessEventStatus, uuid.NewString(), SeedFilter{
		CreatedAt:      time.Now().UTC().Add(-duration),
		DocumentStatus: datastore.ActiveDocumentStatus,
	})
	require.NoError(r.T(), err)

	//call handler
	task := asynq.NewTask(string(convoy.TaskName("retention-policies")), nil, asynq.Queue(string(convoy.ScheduleQueue)))

	fn := RententionPolicies(getConfig(), r.ConvoyApp.configRepo, r.ConvoyApp.groupRepo, r.ConvoyApp.eventRepo, r.ConvoyApp.eventDeliveryRepo, r.ConvoyApp.searcher)
	err = fn(context.Background(), task)
	require.NoError(r.T(), err)

	//check that event and eventdelivery repos are empty
	_, err = r.DB.EventRepo().FindEventByID(context.Background(), event.UID)
	require.ErrorIs(r.T(), err, datastore.ErrEventNotFound)

	_, err = r.DB.EventDeliveryRepo().FindEventDeliveryByID(context.Background(), eventDelivery.UID)
	require.ErrorIs(r.T(), err, datastore.ErrEventDeliveryNotFound)

	//check the number of retained events on groups
	g, err := r.DB.GroupRepo().FetchGroupByID(context.Background(), group.UID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), g.Metadata.RetainedEvents, 1)

}

func (r *RetentionPoliciesIntegrationTestSuite) Test_Should_Export_Zero_Documents() {
	//seed instance configuration
	_, err := seedConfiguration(r.DB)
	require.NoError(r.T(), err)

	//seed group
	groupConfig := &datastore.GroupConfig{
		Signature: &datastore.SignatureConfiguration{
			Header: "X-Convoy-Signature",
			Hash:   "SHA256",
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
	group, err := testdb.SeedGroup(r.DB, uuid.NewString(), uuid.NewString(), "test", datastore.OutgoingGroup, groupConfig)

	require.NoError(r.T(), err)
	//seed event
	event, err := seedEvent(r.DB, uuid.NewString(), group.UID, "", "*", []byte(`{}`), SeedFilter{
		CreatedAt:      time.Now().UTC(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	})
	require.NoError(r.T(), err)

	//seed eventdelivery
	eventDelivery, err := seedEventDelivery(r.DB, uuid.NewString(), event.UID, uuid.NewString(), group.UID, "", datastore.SuccessEventStatus, uuid.NewString(), SeedFilter{
		CreatedAt:      time.Now().UTC(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	})
	require.NoError(r.T(), err)

	//call handler
	task := asynq.NewTask(string(convoy.TaskName("retention-policies")), nil, asynq.Queue(string(convoy.ScheduleQueue)))

	fn := RententionPolicies(getConfig(), r.ConvoyApp.configRepo, r.ConvoyApp.groupRepo, r.ConvoyApp.eventRepo, r.ConvoyApp.eventDeliveryRepo, r.ConvoyApp.searcher)
	err = fn(context.Background(), task)
	require.NoError(r.T(), err)

	//check that event and eventdelivery is not empty
	e, err := r.DB.EventRepo().FindEventByID(context.Background(), event.UID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), e.UID, event.UID)

	ed, err := r.DB.EventDeliveryRepo().FindEventDeliveryByID(context.Background(), eventDelivery.UID)
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

func getDB() datastore.DatabaseClient {

	db, err := mongoStore.New(getConfig())
	if err != nil {
		panic(fmt.Sprintf("failed to connect to db: %v", err))
	}
	_ = os.Setenv("TZ", "") // Use UTC by default :)

	return db.(*mongoStore.Client)
}

func buildApplication() *applicationHandler {

	db := getDB()
	searcher := noopsearcher.NewNoopSearcher()

	app := &applicationHandler{
		groupRepo:         db.GroupRepo(),
		eventRepo:         db.EventRepo(),
		configRepo:        db.ConfigurationRepo(),
		eventDeliveryRepo: db.EventDeliveryRepo(),
		searcher:          searcher,
	}

	return app
}

type applicationHandler struct {
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	configRepo        datastore.ConfigurationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	searcher          searcher.Searcher
}

func seedEvent(db datastore.DatabaseClient, appID string, groupID string, uid, eventType string, data []byte, filter SeedFilter) (*datastore.Event, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	ev := &datastore.Event{
		UID:            uid,
		EventType:      datastore.EventType(eventType),
		Data:           data,
		AppID:          appID,
		GroupID:        groupID,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Unix(filter.CreatedAt.Unix(), 0)),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: filter.DocumentStatus,
	}

	// Seed Data.
	err := db.EventRepo().CreateEvent(context.TODO(), ev)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

func seedEventDelivery(db datastore.DatabaseClient, appID string, eventID string, endpointID string, groupID string, uid string, status datastore.EventDeliveryStatus, subcriptionID string, filter SeedFilter) (*datastore.EventDelivery, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	eventDelivery := &datastore.EventDelivery{
		UID:            uid,
		EventID:        eventID,
		EndpointID:     endpointID,
		Status:         status,
		AppID:          appID,
		SubscriptionID: subcriptionID,
		GroupID:        groupID,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Unix(filter.CreatedAt.Unix(), 0)),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: filter.DocumentStatus,
	}

	// Seed Data.
	err := db.EventDeliveryRepo().CreateEventDelivery(context.TODO(), eventDelivery)
	if err != nil {
		return nil, err
	}

	return eventDelivery, nil
}

func seedConfiguration(db datastore.DatabaseClient) (*datastore.Configuration, error) {
	defaultStorage := &datastore.DefaultStoragePolicy
	defaultStorage.OnPrem.Path = "/tmp/convoy/export/"

	config := &datastore.Configuration{
		UID:                uuid.NewString(),
		IsAnalyticsEnabled: true,
		StoragePolicy:      defaultStorage,
		DocumentStatus:     datastore.ActiveDocumentStatus,
	}

	//Seed Data
	err := db.ConfigurationRepo().CreateConfiguration(context.TODO(), config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

type SeedFilter struct {
	CreatedAt      time.Time
	DocumentStatus datastore.DocumentStatus
}
