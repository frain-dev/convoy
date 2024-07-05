//go:build integration
// +build integration

package testcon

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/listener"
	pg "github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/migrator"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	redisQueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/testcon/containers"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

var once sync.Once
var pDB *pg.Postgres

func setEnv(dbPort int, redisPort int) {
	_ = os.Setenv("CONVOY_DB_HOST", "localhost")
	_ = os.Setenv("CONVOY_REDIS_SCHEME", "redis")
	_ = os.Setenv("CONVOY_REDIS_PORT", strconv.Itoa(redisPort))

	_ = os.Setenv("CONVOY_DB_HOST", "localhost")
	_ = os.Setenv("CONVOY_DB_SCHEME", "postgres")
	_ = os.Setenv("CONVOY_DB_USERNAME", "postgres")
	_ = os.Setenv("CONVOY_DB_PASSWORD", "postgres")
	_ = os.Setenv("CONVOY_DB_DATABASE", "convoy")
	_ = os.Setenv("CONVOY_DB_PORT", strconv.Itoa(dbPort))
}

func getConfig() config.Configuration {
	err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Get()
	if err != nil {
		log.Fatal(err)
	}

	return cfg
}

func setupTestEnv(t *testing.T) (queue.Queuer, error) {
	container, err := containers.CreatePGContainer(t)
	require.NoError(t, err)
	port := getDbPort(t, container)

	rContainer, err := containers.CreateRedisContainer()
	require.NoError(t, err)
	rPort := getRedisPort(t, rContainer)

	setEnv(port, rPort)
	cfg := getConfig()
	db := getDB()

	m := migrator.New(db)
	err = m.Up()
	require.NoError(t, err)

	var ca cache.Cache
	var q queue.Queuer

	opts, err := getQueueOptions()
	require.NoError(t, err)
	q = redisQueue.NewQueue(opts)

	ca, err = cache.NewCache(cfg.Redis)
	assert.NoError(t, err)

	h := hooks.Init()

	projectListener := listener.NewProjectListener(q)
	h.RegisterHook(datastore.ProjectUpdated, projectListener.AfterUpdate)
	projectRepo := pg.NewProjectRepo(db, ca)

	metaEventRepo := pg.NewMetaEventRepo(db, ca)
	endpointListener := listener.NewEndpointListener(q, projectRepo, metaEventRepo)
	eventDeliveryListener := listener.NewEventDeliveryListener(q, projectRepo, metaEventRepo)

	h.RegisterHook(datastore.EndpointCreated, endpointListener.AfterCreate)
	h.RegisterHook(datastore.EndpointUpdated, endpointListener.AfterUpdate)
	h.RegisterHook(datastore.EndpointDeleted, endpointListener.AfterDelete)
	h.RegisterHook(datastore.EventDeliveryUpdated, eventDeliveryListener.AfterUpdate)
	return q, err
}

func getDB() database.Database {
	once.Do(func() {
		db, err := pg.NewDB(getConfig())
		if err != nil {
			panic(fmt.Sprintf("failed to connect to db: %v", err))
		}
		_ = os.Setenv("TZ", "") // Use UTC by default :)

		dbHooks := hooks.Init()
		dbHooks.RegisterHook(datastore.EndpointCreated, func(data interface{}, changelog interface{}) {})

		pDB = db
	})
	return pDB
}

func getQueueOptions() (queue.QueueOptions, error) {
	var opts queue.QueueOptions
	cfg := getConfig()
	redis, err := rdb.NewClient(cfg.Redis.BuildDsn())
	if err != nil {
		return opts, err
	}
	queueNames := map[string]int{
		string(convoy.EventQueue):       3,
		string(convoy.CreateEventQueue): 3,
		string(convoy.ScheduleQueue):    1,
		string(convoy.DefaultQueue):     1,
		string(convoy.StreamQueue):      1,
		string(convoy.MetaEventQueue):   1,
	}
	opts = queue.QueueOptions{
		Names:        queueNames,
		RedisClient:  redis,
		RedisAddress: cfg.Redis.BuildDsn(),
		Type:         string(config.RedisQueueProvider),
	}

	return opts, nil
}

func getDbPort(t *testing.T, container *containers.PostgresContainer) int {
	// postgres://postgres:postgres@localhost:52226/convoy-test-db?sslmode=disable
	portStrings := strings.Split(container.ConnectionString, "/convoy?")
	portStrings = strings.Split(portStrings[0], ":")
	port, err := strconv.Atoi(portStrings[len(portStrings)-1])
	assert.NoError(t, err)
	log.Info("PostgreSQL port: ", port)
	return port
}

func getRedisPort(t *testing.T, container *containers.RedisContainer) int {
	portStrings := strings.Split(container.ConnectionString, ":")
	port, err := strconv.Atoi(portStrings[len(portStrings)-1])
	assert.NoError(t, err)
	log.Info("Redis port: ", port)
	return port
}

func generateConfig() *datastore.Configuration {
	return &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    false,
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Policy:                   "720h",
			IsRetentionPolicyEnabled: true,
		},
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
	}
}

func setupConfig(t *testing.T, db database.Database) {
	configRepo := pg.NewConfigRepo(db)
	cfg := generateConfig()

	_, err := configRepo.LoadConfiguration(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrConfigNotFound))

	require.NoError(t, configRepo.CreateConfiguration(context.Background(), cfg))
}

func setupSourceSubscription(t *testing.T, db database.Database, project *datastore.Project) *datastore.Subscription {
	sourceID := ulid.Make().String()
	vc := &datastore.VerifierConfig{
		Type: datastore.BasicAuthVerifier,
		BasicAuth: &datastore.BasicAuth{
			UserName: "Convoy",
			Password: "Convoy",
		},
	}
	source, err := testdb.SeedSource(db, project, sourceID, ulid.Make().String(), "", vc, "", "")
	require.NoError(t, err)

	endpoint, err := testdb.SeedEndpoint(db, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(t, err)

	sub, err := testdb.SeedSubscription(db, project, ulid.Make().String(), datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(t, err)

	return sub
}
