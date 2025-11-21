package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	rcache "github.com/frain-dev/convoy/cache/redis"
	cmdserver "github.com/frain-dev/convoy/cmd/server"
	cmdworker "github.com/frain-dev/convoy/cmd/worker"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	noopLicenser "github.com/frain-dev/convoy/internal/pkg/license/noop"
	rlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/redis"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/testenv"
)

var (
	infra *testenv.Environment
	// testMutex ensures tests run sequentially to avoid resource conflicts
	testMutex sync.Mutex
)

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	fmt.Fprintf(os.Stderr, "TestMain: Starting test infrastructure setup...\n")
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: Failed to launch test infrastructure: %v\n", err)
		log.Fatalf("Failed to launch test infrastructure: %v", err)
	}

	fmt.Fprintf(os.Stderr, "TestMain: Infrastructure launched successfully\n")
	infra = res

	fmt.Fprintf(os.Stderr, "TestMain: Running tests...\n")
	code := m.Run()

	fmt.Fprintf(os.Stderr, "TestMain: Cleaning up...\n")
	if err := cleanup(); err != nil {
		log.Fatalf("Failed to cleanup test infrastructure: %v", err)
	}

	os.Exit(code)
}

// E2ETestEnv represents the complete E2E test environment
type E2ETestEnv struct {
	T            *testing.T
	App          *cli.App
	Config       config.Configuration
	ServerURL    string
	Project      *datastore.Project
	Organisation *datastore.Organisation
	User         *datastore.User
	APIKey       string
	ctx          context.Context
	cancelWorker context.CancelFunc
	cancelServer context.CancelFunc
}

// SetupE2E initializes the complete E2E test environment with server and worker
func SetupE2E(t *testing.T) *E2ETestEnv {
	t.Helper()

	// Lock to ensure tests run sequentially (prevents resource conflicts)
	testMutex.Lock()

	ctx := context.Background()

	// Create logger
	logger := testenv.NewLogger(t)
	logger.SetLevel(log.ErrorLevel)

	// Reload config for each test to ensure clean state
	err := config.LoadConfig("")
	require.NoError(t, err)

	cfg, err := config.Get()
	require.NoError(t, err)

	// Clone database for this test
	conn, err := infra.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	// Initialize database hooks
	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data interface{}, changelog interface{}) {})

	pg := postgres.NewFromConnection(conn)

	// Create Redis client
	rd, err := infra.NewRedisClient(t, 0)
	require.NoError(t, err)

	// Set up key manager
	_ = os.Setenv("CONVOY_LOCAL_ENCRYPTION_KEY", "test-key")
	km, err := keys.NewLocalKeyManager("test-key")
	require.NoError(t, err)
	if km.IsSet() {
		_, err = km.GetCurrentKeyFromCache()
		require.NoError(t, err)
	}
	err = keys.Set(km)
	require.NoError(t, err)

	// Load CA cert
	err = config.LoadCaCert("", "")
	require.NoError(t, err)

	// Create rdb client for queue
	redis, err := rdb.NewClient(cfg.Redis.BuildDsn())
	require.NoError(t, err)

	// Create cache
	cache := rcache.NewRedisCacheFromClient(rd)

	// Create queue
	queueNames := map[string]int{
		string(convoy.EventQueue):         3,
		string(convoy.CreateEventQueue):   3,
		string(convoy.EventWorkflowQueue): 3,
		string(convoy.ScheduleQueue):      1,
		string(convoy.DefaultQueue):       1,
		string(convoy.StreamQueue):        1,
		string(convoy.MetaEventQueue):     1,
	}

	queueOpts := queue.QueueOptions{
		RedisClient:  redis,
		Names:        queueNames,
		RedisAddress: cfg.Redis.BuildDsn(),
		Type:         string(config.RedisQueueProvider),
	}

	q := redisqueue.NewQueue(queueOpts)

	// Create rate limiter
	limiter := rlimiter.NewLimiterFromRedisClient(rd)

	// Create licenser
	licenser := noopLicenser.NewLicenser()

	// Create cli.App with all dependencies
	app := &cli.App{
		Version:       "test",
		DB:            pg,
		Redis:         rd,
		Queue:         q,
		Logger:        logger,
		Cache:         cache,
		Rate:          limiter,
		Licenser:      licenser,
		TracerBackend: tracer.NoOpBackend{},
	}

	// Seed test data (before starting server/worker)
	user, err := testdb.SeedDefaultUser(pg)
	require.NoError(t, err)

	org, err := testdb.SeedDefaultOrganisation(pg, user)
	require.NoError(t, err)

	project, err := testdb.SeedDefaultProjectWithSSL(pg, org.UID, &datastore.SSLConfiguration{EnforceSecureEndpoints: false})
	require.NoError(t, err)

	role := auth.Role{
		Type:    auth.RoleProjectAdmin,
		Project: project.UID,
	}
	_, apiKey, err := testdb.SeedAPIKey(pg, role, "", "test", "", "")
	require.NoError(t, err)

	// Seed configuration (required by worker)
	_, err = testdb.SeedConfiguration(pg)
	require.NoError(t, err)

	// Use a unique port for each test to avoid conflicts
	// Use a wide range (10000-20000) to minimize collision probability
	serverPort := uint32(10000 + (time.Now().UnixNano() % 10000))
	cfg.Server.HTTP.Port = serverPort

	// Override global config with our test-specific port
	err = config.Override(&cfg)
	require.NoError(t, err)

	serverURL := fmt.Sprintf("http://localhost:%d", serverPort)

	// Start HTTP Server using cmd function with context for cancellation
	serverCtx, cancelServer := context.WithCancel(ctx)
	go func() {
		err := cmdserver.StartConvoyServer(app)
		if err != nil {
			logger.WithError(err).Error("Server error")
		}
	}()

	// Wait for server to start
	waitForServer(t, serverURL, 10*time.Second)

	// Start Worker using cmd function
	workerCtx, cancelWorker := context.WithCancel(serverCtx)
	go func() {
		err := cmdworker.StartWorker(workerCtx, app, cfg)
		if err != nil && err.Error() != "table already registered" {
			logger.WithError(err).Error("Worker error")
		}
	}()

	// Give worker a moment to start
	time.Sleep(1 * time.Second)

	// Cleanup function
	t.Cleanup(func() {
		cancelWorker()
		cancelServer()
		// Give servers time to fully shut down - increased to ensure cleanup
		time.Sleep(2 * time.Second)

		// Reset memory store to prevent "table already registered" errors in next test
		memorystore.DefaultStore.Reset()

		// Unlock mutex to allow next test to run
		testMutex.Unlock()
	})

	return &E2ETestEnv{
		T:            t,
		App:          app,
		Config:       cfg,
		ServerURL:    serverURL,
		Project:      project,
		Organisation: org,
		User:         user,
		APIKey:       apiKey,
		ctx:          workerCtx,
		cancelWorker: cancelWorker,
		cancelServer: cancelServer,
	}
}

// waitForServer waits for the server to be ready
func waitForServer(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("Server did not start within %v", timeout)
}
