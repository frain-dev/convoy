package task

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
)

type UpdateOrganisationStatusIntegrationTestSuite struct {
	suite.Suite
	ConvoyApp   *applicationHandler
	DefaultUser *datastore.User
	DefaultOrg  *datastore.Organisation
}

func (u *UpdateOrganisationStatusIntegrationTestSuite) SetupSuite() {
	u.ConvoyApp = buildApplication(u.T())
}

func (u *UpdateOrganisationStatusIntegrationTestSuite) SetupTest() {
	user, err := testdb.SeedDefaultUser(u.ConvoyApp.database)
	require.NoError(u.T(), err)
	u.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(u.ConvoyApp.database, user)
	require.NoError(u.T(), err)
	u.DefaultOrg = org
}

type testBillingClient struct {
	*billing.MockBillingClient
	subscriptions map[string]billing.BillingSubscription
	errors        map[string]error
}

func newTestBillingClient() *testBillingClient {
	return &testBillingClient{
		MockBillingClient: &billing.MockBillingClient{},
		subscriptions:     make(map[string]billing.BillingSubscription),
		errors:            make(map[string]error),
	}
}

func (m *testBillingClient) GetSubscription(ctx context.Context, orgID string) (*billing.Response[billing.BillingSubscription], error) {
	if err, ok := m.errors[orgID]; ok {
		return nil, err
	}

	if sub, ok := m.subscriptions[orgID]; ok {
		return &billing.Response[billing.BillingSubscription]{
			Status:  true,
			Message: "Subscription retrieved successfully",
			Data:    sub,
		}, nil
	}

	return m.MockBillingClient.GetSubscription(ctx, orgID)
}

func (u *UpdateOrganisationStatusIntegrationTestSuite) Test_UpdateOrganisationStatus_WithActiveSubscription() {
	org1, err := testdb.SeedDefaultOrganisation(u.ConvoyApp.database, u.DefaultUser)
	require.NoError(u.T(), err)

	org2, err := testdb.SeedDefaultOrganisation(u.ConvoyApp.database, u.DefaultUser)
	require.NoError(u.T(), err)

	org2.DisabledAt = null.NewTime(time.Now(), true)
	orgRepo := organisations.New(log.NewLogger(os.Stdout), u.ConvoyApp.database)
	err = orgRepo.UpdateOrganisation(context.Background(), org2)
	require.NoError(u.T(), err)

	testClient := newTestBillingClient()
	testClient.subscriptions[u.DefaultOrg.UID] = billing.BillingSubscription{
		Status: "active",
	}
	testClient.subscriptions[org1.UID] = billing.BillingSubscription{
		Status: "active",
	}
	testClient.subscriptions[org2.UID] = billing.BillingSubscription{
		Status: "inactive",
	}

	cfg, err := config.Get()
	require.NoError(u.T(), err)
	cfg.Billing.Enabled = true
	config.Override(&cfg)

	rd, err := rdb.NewClientFromConfig(
		cfg.Redis.BuildDsn(),
		cfg.Redis.TLSSkipVerify,
		cfg.Redis.TLSCACertFile,
		cfg.Redis.TLSCertFile,
		cfg.Redis.TLSKeyFile,
	)
	require.NoError(u.T(), err)

	logger := log.NewLogger(os.Stdout)
	fn := UpdateOrganisationStatus(u.ConvoyApp.database, testClient, rd, logger)

	task := asynq.NewTask(string(convoy.UpdateOrganisationStatus), nil, asynq.Queue(string(convoy.ScheduleQueue)))
	err = fn(context.Background(), task)
	require.NoError(u.T(), err)

	updatedOrg1, err := orgRepo.FetchOrganisationByID(context.Background(), org1.UID)
	require.NoError(u.T(), err)
	require.False(u.T(), updatedOrg1.DisabledAt.Valid, "org1 should not be disabled")

	updatedOrg2, err := orgRepo.FetchOrganisationByID(context.Background(), org2.UID)
	require.NoError(u.T(), err)
	require.True(u.T(), updatedOrg2.DisabledAt.Valid, "org2 should be disabled")
}

func (u *UpdateOrganisationStatusIntegrationTestSuite) Test_UpdateOrganisationStatus_EnableDisabledOrg() {
	org, err := testdb.SeedDefaultOrganisation(u.ConvoyApp.database, u.DefaultUser)
	require.NoError(u.T(), err)

	org.DisabledAt = null.NewTime(time.Now(), true)
	orgRepo := organisations.New(log.NewLogger(os.Stdout), u.ConvoyApp.database)
	err = orgRepo.UpdateOrganisation(context.Background(), org)
	require.NoError(u.T(), err)

	testClient := newTestBillingClient()
	testClient.subscriptions[u.DefaultOrg.UID] = billing.BillingSubscription{
		Status: "active",
	}
	testClient.subscriptions[org.UID] = billing.BillingSubscription{
		Status: "active",
	}

	cfg, err := config.Get()
	require.NoError(u.T(), err)
	cfg.Billing.Enabled = true
	config.Override(&cfg)

	rd, err := rdb.NewClientFromConfig(
		cfg.Redis.BuildDsn(),
		cfg.Redis.TLSSkipVerify,
		cfg.Redis.TLSCACertFile,
		cfg.Redis.TLSCertFile,
		cfg.Redis.TLSKeyFile,
	)
	require.NoError(u.T(), err)

	logger := log.NewLogger(os.Stdout)
	fn := UpdateOrganisationStatus(u.ConvoyApp.database, testClient, rd, logger)

	task := asynq.NewTask(string(convoy.UpdateOrganisationStatus), nil, asynq.Queue(string(convoy.ScheduleQueue)))
	err = fn(context.Background(), task)
	require.NoError(u.T(), err)

	updatedOrg, err := orgRepo.FetchOrganisationByID(context.Background(), org.UID)
	require.NoError(u.T(), err)
	require.False(u.T(), updatedOrg.DisabledAt.Valid, "org should be enabled")
}

func (u *UpdateOrganisationStatusIntegrationTestSuite) Test_UpdateOrganisationStatus_DisableEnabledOrg() {
	org, err := testdb.SeedDefaultOrganisation(u.ConvoyApp.database, u.DefaultUser)
	require.NoError(u.T(), err)

	orgRepo := organisations.New(log.NewLogger(os.Stdout), u.ConvoyApp.database)

	testClient := newTestBillingClient()
	testClient.subscriptions[u.DefaultOrg.UID] = billing.BillingSubscription{
		Status: "active",
	}
	testClient.subscriptions[org.UID] = billing.BillingSubscription{
		Status: "inactive",
	}

	cfg, err := config.Get()
	require.NoError(u.T(), err)
	cfg.Billing.Enabled = true
	config.Override(&cfg)

	rd, err := rdb.NewClientFromConfig(
		cfg.Redis.BuildDsn(),
		cfg.Redis.TLSSkipVerify,
		cfg.Redis.TLSCACertFile,
		cfg.Redis.TLSCertFile,
		cfg.Redis.TLSKeyFile,
	)
	require.NoError(u.T(), err)

	logger := log.NewLogger(os.Stdout)
	fn := UpdateOrganisationStatus(u.ConvoyApp.database, testClient, rd, logger)

	task := asynq.NewTask(string(convoy.UpdateOrganisationStatus), nil, asynq.Queue(string(convoy.ScheduleQueue)))
	err = fn(context.Background(), task)
	require.NoError(u.T(), err)

	updatedOrg, err := orgRepo.FetchOrganisationByID(context.Background(), org.UID)
	require.NoError(u.T(), err)
	require.True(u.T(), updatedOrg.DisabledAt.Valid, "org should be disabled")
}

func (u *UpdateOrganisationStatusIntegrationTestSuite) Test_UpdateOrganisationStatus_SkipsWhenBillingDisabled() {
	cfg, err := config.Get()
	require.NoError(u.T(), err)
	cfg.Billing.Enabled = false
	config.Override(&cfg)

	rd, err := rdb.NewClientFromConfig(
		cfg.Redis.BuildDsn(),
		cfg.Redis.TLSSkipVerify,
		cfg.Redis.TLSCACertFile,
		cfg.Redis.TLSCertFile,
		cfg.Redis.TLSKeyFile,
	)
	require.NoError(u.T(), err)

	logger := log.NewLogger(nil)
	testClient := newTestBillingClient()
	fn := UpdateOrganisationStatus(u.ConvoyApp.database, testClient, rd, logger)

	task := asynq.NewTask(string(convoy.UpdateOrganisationStatus), nil, asynq.Queue(string(convoy.ScheduleQueue)))
	err = fn(context.Background(), task)
	require.NoError(u.T(), err)
}

func (u *UpdateOrganisationStatusIntegrationTestSuite) Test_UpdateOrganisationStatus_HandlesBillingClientError() {
	org1, err := testdb.SeedDefaultOrganisation(u.ConvoyApp.database, u.DefaultUser)
	require.NoError(u.T(), err)

	org2, err := testdb.SeedDefaultOrganisation(u.ConvoyApp.database, u.DefaultUser)
	require.NoError(u.T(), err)

	testClient := newTestBillingClient()
	testClient.subscriptions[u.DefaultOrg.UID] = billing.BillingSubscription{
		Status: "active",
	}
	testClient.errors[org1.UID] = errors.New("billing service error")
	testClient.subscriptions[org2.UID] = billing.BillingSubscription{
		Status: "active",
	}

	cfg, err := config.Get()
	require.NoError(u.T(), err)
	cfg.Billing.Enabled = true
	config.Override(&cfg)

	rd, err := rdb.NewClientFromConfig(
		cfg.Redis.BuildDsn(),
		cfg.Redis.TLSSkipVerify,
		cfg.Redis.TLSCACertFile,
		cfg.Redis.TLSCertFile,
		cfg.Redis.TLSKeyFile,
	)
	require.NoError(u.T(), err)

	logger := log.NewLogger(os.Stdout)
	fn := UpdateOrganisationStatus(u.ConvoyApp.database, testClient, rd, logger)

	task := asynq.NewTask(string(convoy.UpdateOrganisationStatus), nil, asynq.Queue(string(convoy.ScheduleQueue)))
	err = fn(context.Background(), task)
	require.NoError(u.T(), err)

	orgRepo := organisations.New(log.NewLogger(os.Stdout), u.ConvoyApp.database)
	updatedOrg2, err := orgRepo.FetchOrganisationByID(context.Background(), org2.UID)
	require.NoError(u.T(), err)
	require.False(u.T(), updatedOrg2.DisabledAt.Valid, "org2 should be enabled despite org1 error")
}

func TestUpdateOrganisationStatusIntegrationSuiteTest(t *testing.T) {
	suite.Run(t, new(UpdateOrganisationStatusIntegrationTestSuite))
}
