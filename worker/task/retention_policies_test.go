package task

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/guregu/null.v4"

	partman "github.com/jirevwe/go_partman"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/delivery_attempts"
	"github.com/frain-dev/convoy/internal/pkg/retention"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type RetentionPoliciesIntegrationTestSuite struct {
	suite.Suite
	ConvoyApp   *applicationHandler
	DefaultUser *datastore.User
	DefaultOrg  *datastore.Organisation
}

func (r *RetentionPoliciesIntegrationTestSuite) SetupSuite() {
	r.ConvoyApp = buildApplication(r.T())
}

func (r *RetentionPoliciesIntegrationTestSuite) SetupTest() {
	user, err := testdb.SeedDefaultUser(r.ConvoyApp.database)
	require.NoError(r.T(), err)
	r.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(r.ConvoyApp.database, user)
	require.NoError(r.T(), err)
	r.DefaultOrg = org
}

// todo(raymond):
//  1. update this test case such that we verify that rows not in the current window are not deleted
//  2. add test case to verify that events created before partitioning the table are backed up and retained
//  3. update the Setup and Teardown helpers to clear partition manager table to prevent bloat
func (r *RetentionPoliciesIntegrationTestSuite) Test_Should_Export_Two_Documents() {
	// seed event
	duration := time.Hour * 24

	// seed instance configuration
	_, err := seedConfiguration(r.ConvoyApp.database)
	require.NoError(r.T(), err)

	err = r.ConvoyApp.eventRepo.PartitionEventsTable(context.Background())
	require.NoError(r.T(), err)

	err = r.ConvoyApp.eventDeliveryRepo.PartitionEventDeliveriesTable(context.Background())
	require.NoError(r.T(), err)

	err = r.ConvoyApp.deliveryRepo.PartitionDeliveryAttemptsTable(context.Background())
	require.NoError(r.T(), err)

	defer func() {
		err = r.ConvoyApp.eventRepo.UnPartitionEventsTable(context.Background())
		require.NoError(r.T(), err)

		err = r.ConvoyApp.eventDeliveryRepo.UnPartitionEventDeliveriesTable(context.Background())
		require.NoError(r.T(), err)

		err = r.ConvoyApp.deliveryRepo.UnPartitionDeliveryAttemptsTable(context.Background())
		require.NoError(r.T(), err)
	}()

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
	}
	project, err := testdb.SeedProject(r.ConvoyApp.database, ulid.Make().String(), "test", r.DefaultOrg.UID, datastore.OutgoingProject, projectConfig)
	require.NoError(r.T(), err)

	pmConfig := &partman.Config{
		SampleRate: time.Second,
		Tables: []partman.Table{
			{
				Name:              "events",
				Schema:            "convoy",
				TenantId:          project.UID,
				TenantIdColumn:    "project_id",
				PartitionBy:       "created_at",
				PartitionType:     partman.TypeRange,
				RetentionPeriod:   time.Hour * 24,
				PartitionInterval: time.Hour * 24,
				PartitionCount:    2,
			},
			{
				Name:              "event_deliveries",
				Schema:            "convoy",
				TenantId:          project.UID,
				TenantIdColumn:    "project_id",
				PartitionBy:       "created_at",
				PartitionType:     partman.TypeRange,
				RetentionPeriod:   time.Hour * 24,
				PartitionInterval: time.Hour * 24,
				PartitionCount:    2,
			},
			{
				Name:              "delivery_attempts",
				Schema:            "convoy",
				TenantId:          project.UID,
				TenantIdColumn:    "project_id",
				PartitionBy:       "created_at",
				PartitionType:     partman.TypeRange,
				RetentionPeriod:   time.Hour * 24,
				PartitionInterval: time.Hour * 24,
				PartitionCount:    2,
			},
		},
	}

	clock := partman.NewSimulatedClock(time.Now().Add(-duration))
	pm, err := partman.NewManager(
		partman.WithConfig(pmConfig),
		partman.WithDB(r.ConvoyApp.database.GetDB()),
		partman.WithClock(clock),
		partman.WithLogger(log.NewLogger(os.Stdout)),
	)
	require.NoError(r.T(), err)

	ret := retention.NewTestRetentionPolicy(r.ConvoyApp.database, pm)
	ret.Start(context.Background(), time.Second)

	endpoint, err := testdb.SeedEndpoint(r.ConvoyApp.database, project, ulid.Make().String(), "test-endpoint", "", false, datastore.ActiveEndpointStatus)
	require.NoError(r.T(), err)

	event1, err := seedEvent(r.ConvoyApp.database, endpoint.UID, project.UID, "", "*", []byte(`{}`), SeedFilter{
		CreatedAt: clock.Now(),
	})
	require.NoError(r.T(), err)

	event2, err := seedEvent(r.ConvoyApp.database, endpoint.UID, project.UID, "", "*", []byte(`{}`), SeedFilter{
		CreatedAt: clock.Now(),
	})
	require.NoError(r.T(), err)

	subscription, err := testdb.SeedSubscription(r.ConvoyApp.database, project, "", project.Type, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(r.T(), err)

	now := clock.Now().UTC()
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
	retentionTask := asynq.NewTask(string(convoy.RetentionPolicies), nil, asynq.Queue(string(convoy.ScheduleQueue)))
	backUpTask := asynq.NewTask(string(convoy.BackupProjectData), nil, asynq.Queue(string(convoy.ScheduleQueue)))

	clock.AdvanceTime(duration + time.Hour)

	err = BackupProjectData(
		r.ConvoyApp.configRepo,
		r.ConvoyApp.projectRepo,
		r.ConvoyApp.eventRepo,
		r.ConvoyApp.eventDeliveryRepo,
		r.ConvoyApp.deliveryRepo,
		r.ConvoyApp.redis)(context.Background(), backUpTask)
	require.NoError(r.T(), err)

	err = RetentionPolicies(r.ConvoyApp.redis, ret)(context.Background(), retentionTask)
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
}

func TestRetentionPoliciesIntegrationSuiteTest(t *testing.T) {
	suite.Run(t, new(RetentionPoliciesIntegrationTestSuite))
}

type applicationHandler struct {
	projectRepo       datastore.ProjectRepository
	eventRepo         datastore.EventRepository
	configRepo        datastore.ConfigurationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	deliveryRepo      datastore.DeliveryAttemptsRepository
	database          database.Database
	redis             redis.UniversalClient
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
		DeliveryMode:   datastore.AtLeastOnceDeliveryMode,
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

	daRepo := delivery_attempts.New(log.NewLogger(os.Stdout), db)
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
	}

	// Seed Data
	configRepo := configuration.New(log.NewLogger(os.Stdout), db)
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
