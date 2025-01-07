//go:build integration
// +build integration

package api

import (
	"context"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"
	"os"
	"testing"
)

func Test_CircuitBreakerManagerWorks(t *testing.T) {
	db := getDB()
	testdb.PurgeDB(t, db)

	client, err := getRedis(t)
	require.NoError(t, err)

	config := generateConfig()

	configRepo := postgres.NewConfigRepo(db)
	attemptRepo := postgres.NewDeliveryAttemptRepo(db)
	projectRepo := postgres.NewProjectRepo(db)

	user, err := testdb.SeedDefaultUser(db)
	require.NoError(t, err)

	org, err := testdb.SeedDefaultOrganisation(db, user)
	require.NoError(t, err)

	project, err := testdb.SeedDefaultProject(db, org.UID)
	require.NoError(t, err)

	require.NoError(t, configRepo.CreateConfiguration(context.Background(), config))

	require.NoError(t, err)

	circuitBreakerManager, err := cb.NewCircuitBreakerManager(
		cb.ConfigOption(config.ToCircuitBreakerConfig()),
		cb.StoreOption(cb.NewRedisStore(client, clock.NewRealClock())),
		cb.ClockOption(clock.NewRealClock()),
		cb.LoggerOption(log.NewLogger(os.Stdout)),
		cb.NotificationFunctionOption(func(_ cb.NotificationType, _ cb.CircuitBreakerConfig, _ cb.CircuitBreaker) error {
			return nil
		}))
	require.NoError(t, err)

	err = circuitBreakerManager.SampleAndUpdate(context.Background(), attemptRepo.GetFailureAndSuccessCounts)
	require.NoError(t, err)

	err = circuitBreakerManager.RefreshCircuitBreakerConfigs(context.Background(), projectRepo.FetchCircuitBreakerConfigsFromProjects)
	require.NoError(t, err)

	totalEndpoints := 15
	endpoints, err := testdb.SeedMultipleEndpoints(db, project, totalEndpoints)
	require.NoError(t, err)

	for _, endpoint := range endpoints {
		event, _ := testdb.SeedEvent(db, endpoint, project.UID, ulid.Make().String(), "*", "", []byte(`{}`))
		subscription, err := testdb.SeedSubscription(db, project, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
			Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
		})
		require.NoError(t, err)

		ed, err := testdb.SeedEventDelivery(db, event, endpoint, project.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
		require.NoError(t, err)

		attempts := []datastore.DeliveryAttempt{
			{
				UID:              ulid.Make().String(),
				EventDeliveryId:  ed.UID,
				URL:              "https://example.com",
				Method:           "POST",
				EndpointID:       endpoint.UID,
				ProjectId:        project.UID,
				APIVersion:       "2024-01-01",
				IPAddress:        "192.168.0.1",
				RequestHeader:    map[string]string{"Content-Type": "application/json"},
				ResponseHeader:   map[string]string{"Content-Type": "application/json"},
				HttpResponseCode: "200",
				ResponseData:     []byte("{\"status\":\"ok\"}"),
				Status:           true,
			},
			{
				UID:              ulid.Make().String(),
				EventDeliveryId:  ed.UID,
				URL:              "https://main.com",
				Method:           "POST",
				EndpointID:       endpoint.UID,
				ProjectId:        project.UID,
				APIVersion:       "2024-04-04",
				IPAddress:        "127.0.0.1",
				RequestHeader:    map[string]string{"Content-Type": "application/json"},
				ResponseHeader:   map[string]string{"Content-Type": "application/json"},
				HttpResponseCode: "400",
				ResponseData:     []byte("{\"status\":\"Not Found\"}"),
				Error:            "",
				Status:           false,
			},
		}

		for _, a := range attempts {
			err = attemptRepo.CreateDeliveryAttempt(context.Background(), &a)
			require.NoError(t, err)
		}
	}

	err = circuitBreakerManager.SampleAndUpdate(context.Background(), attemptRepo.GetFailureAndSuccessCounts)
	require.NoError(t, err)

	err = circuitBreakerManager.RefreshCircuitBreakerConfigs(context.Background(), projectRepo.FetchCircuitBreakerConfigsFromProjects)
	require.NoError(t, err)
}

func getRedis(t *testing.T) (client redis.UniversalClient, err error) {
	t.Helper()

	opts, err := redis.ParseURL("redis://localhost:6379")
	if err != nil {
		return nil, err
	}

	return redis.NewClient(opts), nil
}

func generateConfig() *datastore.Configuration {
	return &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    false,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: datastore.OnPrem,
			S3: &datastore.S3Storage{
				Prefix:       null.NewString("random7", true),
				Bucket:       null.NewString("random1", true),
				AccessKey:    null.NewString("random2", true),
				SecretKey:    null.NewString("random3", true),
				Region:       null.NewString("random4", true),
				SessionToken: null.NewString("random5", true),
				Endpoint:     null.NewString("random6", true),
			},
			OnPrem: &datastore.OnPremStorage{
				Path: null.NewString("path", true),
			},
		},
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy:                   "720h",
			IsRetentionPolicyEnabled: true,
		},
		CircuitBreakerConfig: &datastore.DefaultCircuitBreakerConfiguration,
	}
}
