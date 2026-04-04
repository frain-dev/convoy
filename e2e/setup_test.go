package e2e

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	rcache "github.com/frain-dev/convoy/cache/redis"
	cmdserver "github.com/frain-dev/convoy/cmd/server"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/dataplane"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	noopLicenser "github.com/frain-dev/convoy/internal/pkg/license/noop"
	rlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/redis"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/testenv"
)

var (
	infra *testenv.Environment
	// testMutex ensures tests run sequentially to avoid resource conflicts
	testMutex sync.Mutex
	// portCounter ensures unique ports across tests (starts at 15000)
	portCounter atomic.Uint32
)

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Starting E2E test infrastructure setup...\n")
	// Standard E2E tests only need Postgres + Redis (defaults)
	// Message broker and storage-specific tests are in separate packages (amqp, kafka, sqs, pubsub, backup)
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "TestMain: Failed to launch test infrastructure: %v\n", err)
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: E2E infrastructure launched successfully\n")
	infra = res

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Running tests...\n")
	code := m.Run()

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Cleaning up...\n")
	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup test infrastructure: %v\n", err)
		os.Exit(1)
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
	JobTracker   *testenv.JobTracker
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

	// Flush Redis to ensure no stale data from previous tests
	err = rd.FlushDB(ctx).Err()
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

	// Always flush Redis to ensure complete test isolation.
	// Each test has its own database clone, so we MUST ensure no jobs from
	// previous tests leak through the shared Redis queue.
	t.Logf("Flushing Redis to ensure clean state for test %s", t.Name())
	err = redis.Client().FlushDB(ctx).Err()
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

	// Create job tracker for E2E tests to capture job IDs
	jobTracker := testenv.NewJobTracker()

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
		JobTracker:    jobTracker,
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
	// Use atomic counter to ensure no port collisions
	nextPort := portCounter.Add(1)
	if nextPort == 1 {
		// First test - initialize to a safe starting port
		portCounter.Store(15000)
		nextPort = 15000
	}
	serverPort := nextPort
	cfg.Server.HTTP.Port = serverPort

	// Override global config with our test-specific settings
	err = config.Override(&cfg)
	require.NoError(t, err)

	serverURL := fmt.Sprintf("http://localhost:%d", serverPort)

	// Start HTTP Server using cmd function with context for cancellation
	serverCtx, cancelServer := context.WithCancel(ctx)
	go func() {
		err := cmdserver.StartConvoyServer(app)
		if err != nil {
			logger.Error("Server error", "error", err)
		}
	}()

	// Wait for server to start
	waitForServer(t, serverURL, 10*time.Second)

	// Start Worker using cmd function
	workerCtx, cancelWorker := context.WithCancel(serverCtx)
	go func() {
		t.Logf("Starting worker for test: %s", t.Name())
		worker, err := dataplane.NewWorker(workerCtx, dataplaneDeps(app), cfg)
		if err != nil {
			t.Logf("Worker initialization error for test %s: %v", t.Name(), err)
			logger.Error("Worker initialization error", "error", err)
			return
		}
		err = worker.Run(workerCtx, nil)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Logf("Worker error for test %s: %v", t.Name(), err)
				logger.Error("Worker error", "error", err)
			} else {
				t.Logf("Worker context canceled (expected during cleanup)")
			}
		} else {
			t.Logf("Worker exited with nil error (unexpected!)")
		}
		t.Logf("Worker stopped for test: %s", t.Name())
	}()

	// Give worker more time to fully start and begin processing
	time.Sleep(3 * time.Second)
	t.Logf("Test environment ready: %s (port: %d, project: %s)", t.Name(), serverPort, project.UID)

	// Cleanup function
	t.Cleanup(func() {
		t.Logf("Cleaning up test: %s", t.Name())

		// Cancel worker and server contexts
		cancelWorker()
		cancelServer()

		// Wait for worker to finish processing any in-flight jobs and fully shut down
		// Asynq consumer needs time to gracefully stop all goroutines and release Redis connections
		t.Logf("Waiting for worker to fully shut down...")
		time.Sleep(10 * time.Second)

		// Note: Redis is flushed at the START of each test (not in cleanup),
		// so the next test begins with clean state.

		// Reset memory store to prevent "table already registered" errors in next test
		memorystore.DefaultStore.Reset()
		t.Logf("Memorystore reset complete")

		// Unlock mutex to allow next test to run
		testMutex.Unlock()
		t.Logf("Cleanup complete for test: %s", t.Name())
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
		JobTracker:   jobTracker,
		ctx:          workerCtx,
		cancelWorker: cancelWorker,
		cancelServer: cancelServer,
	}
}

// SetupE2EWithoutWorker initializes E2E test environment with server but WITHOUT worker
// This is useful for job ID tests where we use a custom test worker instead
func SetupE2EWithoutWorker(t *testing.T) *E2ETestEnv {
	t.Helper()

	// Lock to ensure tests run sequentially (prevents resource conflicts)
	testMutex.Lock()

	ctx := context.Background()

	// Create logger
	logger := testenv.NewLogger(t)

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

	// Flush Redis to ensure no stale data from previous tests
	err = rd.FlushDB(ctx).Err()
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

	// Always flush Redis to ensure complete test isolation.
	// Each test has its own database clone, so we MUST ensure no jobs from
	// previous tests leak through the shared Redis queue.
	t.Logf("Flushing Redis to ensure clean state for test %s", t.Name())
	err = redis.Client().FlushDB(ctx).Err()
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

	// Seed test data (before starting server)
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
	// Use atomic counter to ensure no port collisions
	nextPort := portCounter.Add(1)
	if nextPort == 1 {
		// First test - initialize to a safe starting port
		portCounter.Store(15000)
		nextPort = 15000
	}
	serverPort := nextPort
	cfg.Server.HTTP.Port = serverPort

	// Override global config with our test-specific settings
	err = config.Override(&cfg)
	require.NoError(t, err)

	serverURL := fmt.Sprintf("http://localhost:%d", serverPort)

	// Start HTTP Server using cmd function with context for cancellation
	serverCtx, cancelServer := context.WithCancel(ctx)
	go func() {
		err := cmdserver.StartConvoyServer(app)
		if err != nil {
			logger.Error("Server error", "error", err)
		}
	}()

	// Wait for server to start
	waitForServer(t, serverURL, 10*time.Second)

	// NOTE: We do NOT start the real worker here
	// Tests using this setup should start their own custom test worker

	t.Logf("Test environment ready (without worker): %s (port: %d, project: %s)", t.Name(), serverPort, project.UID)

	// Cleanup function
	t.Cleanup(func() {
		t.Logf("Cleaning up test: %s", t.Name())
		cancelServer()
		// Give server time to fully shut down
		time.Sleep(2 * time.Second)

		// Reset memory store to prevent "table already registered" errors in next test
		memorystore.DefaultStore.Reset()

		// Unlock mutex to allow next test to run
		testMutex.Unlock()
		t.Logf("Cleanup complete for test: %s", t.Name())
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
		JobTracker:   nil, // Not used for custom worker tests
		ctx:          serverCtx,
		cancelWorker: nil,
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

func dataplaneDeps(app *cli.App) dataplane.RuntimeDeps {
	return dataplane.RuntimeDeps{
		DB:            app.DB,
		Redis:         app.Redis,
		Queue:         app.Queue,
		Logger:        app.Logger,
		Cache:         app.Cache,
		Rate:          app.Rate,
		Licenser:      app.Licenser,
		TracerBackend: app.TracerBackend,
		JobTracker:    app.JobTracker,
		SetSubscriptionLoader: func(loader interface{}) {
			app.SubscriptionLoader = loader
		},
		SetSubscriptionTable: func(table interface{}) {
			app.SubscriptionTable = table
		},
	}
}
