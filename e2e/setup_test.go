package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	cmdworker "github.com/frain-dev/convoy/cmd/worker"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	noopLicenser "github.com/frain-dev/convoy/internal/pkg/license/noop"
	rlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/redis"
	"github.com/frain-dev/convoy/internal/pkg/loader"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/sources"
	"github.com/frain-dev/convoy/pkg/log"
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
	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Starting test infrastructure setup...\n")
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "TestMain: Failed to launch test infrastructure: %v\n", err)
		log.Fatalf("Failed to launch test infrastructure: %v", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Infrastructure launched successfully\n")
	infra = res

	// Set PUBSUB_EMULATOR_HOST for all tests that need Google Pub/Sub emulator
	// This must be set BEFORE any test runs so the pubsub package initialization sees it
	if res.NewPubSubEmulatorHost != nil {
		emulatorHost := res.NewPubSubEmulatorHost(nil) // Pass nil since we modified factory to handle it
		os.Setenv("PUBSUB_EMULATOR_HOST", emulatorHost)
		_, _ = fmt.Fprintf(os.Stderr, "TestMain: Set PUBSUB_EMULATOR_HOST=%s\n", emulatorHost)
		// Verify it was set
		verifyHost := os.Getenv("PUBSUB_EMULATOR_HOST")
		_, _ = fmt.Fprintf(os.Stderr, "TestMain: Verified PUBSUB_EMULATOR_HOST=%s\n", verifyHost)
	}

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Running tests...\n")
	code := m.Run()

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Cleaning up...\n")
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
			logger.WithError(err).Error("Server error")
		}
	}()

	// Wait for server to start
	waitForServer(t, serverURL, 10*time.Second)

	// Start Worker using cmd function
	workerCtx, cancelWorker := context.WithCancel(serverCtx)
	go func() {
		t.Logf("Starting worker for test: %s", t.Name())
		worker, err := cmdworker.NewWorker(workerCtx, app, cfg)
		if err != nil {
			t.Logf("Worker initialization error for test %s: %v", t.Name(), err)
			logger.WithError(err).Error("Worker initialization error")
			return
		}
		err = worker.Run(workerCtx, nil)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Logf("Worker error for test %s: %v", t.Name(), err)
				logger.WithError(err).Error("Worker error")
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
			logger.WithError(err).Error("Server error")
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

// E2ETestEnvWithAMQP extends E2ETestEnv with AMQP-specific configuration
type E2ETestEnvWithAMQP struct {
	*E2ETestEnv
	RabbitMQHost  string
	RabbitMQPort  int
	Ingest        *pubsub.Ingest
	cancelIngest  context.CancelFunc
	sourceTable   *memorystore.Table
	sourceContext context.Context
}

// SetupE2EWithAMQP initializes the E2E test environment with AMQP/RabbitMQ support
func SetupE2EWithAMQP(t *testing.T) *E2ETestEnvWithAMQP {
	t.Helper()

	// Reset memorystore to ensure test isolation from previous tests
	// This must be done BEFORE setting up the base environment which starts the worker
	memorystore.DefaultStore.Reset()
	t.Logf("Reset memorystore before test setup for isolation")

	// First set up the base E2E environment
	baseEnv := SetupE2E(t)

	// Get RabbitMQ connection details
	rmqHost, rmqPort, err := infra.NewRabbitMQConnect(t)
	require.NoError(t, err)

	// Set up repositories needed for source loading
	sourceRepo := sources.New(log.NewLogger(io.Discard), baseEnv.App.DB)
	endpointRepo := postgres.NewEndpointRepo(baseEnv.App.DB)
	projectRepo := projects.New(baseEnv.App.Logger, baseEnv.App.DB)

	// Create the source loader and table for pubsub ingest
	lo := baseEnv.App.Logger.(*log.Logger)
	sourceLoader := pubsub.NewSourceLoader(endpointRepo, sourceRepo, projectRepo, lo)
	sourceTable := memorystore.NewTable(memorystore.OptionSyncer(sourceLoader))

	// Register the sources table with the memory store
	err = memorystore.DefaultStore.Register("sources", sourceTable)
	if err != nil {
		// If already registered (from a previous test), that's okay
		t.Logf("Source table registration: %v", err)
	}

	// Note: We DON'T create a separate subscription loader here because the worker
	// creates its own and exposes it via App.SubscriptionLoader. Creating multiple
	// loaders causes test isolation issues.

	// Create the pubsub ingest component
	ingestComponent, err := pubsub.NewIngest(
		baseEnv.ctx,
		sourceTable,
		baseEnv.App.Queue,
		baseEnv.App.Logger,
		baseEnv.App.Rate,
		baseEnv.App.Licenser,
		"test-instance",
		endpointRepo,
	)
	require.NoError(t, err)

	// Start the ingest component in a goroutine
	_, cancelIngest := context.WithCancel(baseEnv.ctx)
	t.Logf("Starting ingest component in goroutine")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("PANIC in ingest component: %v", r)
			}
		}()
		t.Logf("Ingest goroutine started, calling Run()")
		ingestComponent.Run()
		t.Logf("Ingest Run() returned")
	}()

	// Give the ingest component time to start and sync the sources
	t.Logf("Waiting for ingest component to start...")
	time.Sleep(1 * time.Second)
	t.Logf("Ingest component should be running now")

	// Add cleanup for ingest component
	t.Cleanup(func() {
		t.Logf("Stopping AMQP ingest component for test: %s", t.Name())
		cancelIngest()
		// Small delay to allow cleanup
		time.Sleep(200 * time.Millisecond)
	})

	amqpEnv := &E2ETestEnvWithAMQP{
		E2ETestEnv:    baseEnv,
		RabbitMQHost:  rmqHost,
		RabbitMQPort:  rmqPort,
		Ingest:        ingestComponent,
		cancelIngest:  cancelIngest,
		sourceTable:   sourceTable,
		sourceContext: baseEnv.ctx,
	}

	return amqpEnv
}

// SyncSources manually triggers a sync of the sources table to pick up new sources
func (env *E2ETestEnvWithAMQP) SyncSources(t *testing.T) {
	t.Helper()
	err := env.sourceTable.Sync(env.sourceContext)
	if err != nil {
		t.Logf("Warning: failed to sync sources: %v", err)
	}
	// Give the ingest component time to process the new sources
	time.Sleep(1 * time.Second)
}

// SyncSubscriptions forces an immediate sync of the worker's subscription loader
// This is needed for broadcast events which use in-memory subscription lookup
func (env *E2ETestEnvWithAMQP) SyncSubscriptions(t *testing.T) {
	t.Helper()

	// Access the worker's subscription loader from the App
	if env.App.SubscriptionLoader == nil {
		t.Fatal("SubscriptionLoader not initialized in App - worker may not have started")
	}

	subLoader, ok := env.App.SubscriptionLoader.(*loader.SubscriptionLoader)
	if !ok {
		t.Fatal("SubscriptionLoader is not of expected type")
	}

	subTable, ok := env.App.SubscriptionTable.(*memorystore.Table)
	if !ok {
		t.Fatal("SubscriptionTable is not of expected type")
	}

	// Force an immediate sync of the worker's subscription loader
	err := subLoader.SyncChanges(env.ctx, subTable)
	if err != nil {
		t.Logf("Warning: failed to sync subscriptions: %v", err)
	}
	t.Logf("Forced worker subscription loader sync completed")

	// Give a small delay for the changes to propagate
	time.Sleep(500 * time.Millisecond)
}

// E2ETestEnvWithSQS extends E2ETestEnv with SQS-specific resources
type E2ETestEnvWithSQS struct {
	*E2ETestEnv
	LocalStackEndpoint string
	Ingest             *pubsub.Ingest
	cancelIngest       context.CancelFunc
	sourceTable        *memorystore.Table
	sourceContext      context.Context
}

// SetupE2EWithSQS sets up an E2E test environment with SQS pubsub infrastructure
func SetupE2EWithSQS(t *testing.T) *E2ETestEnvWithSQS {
	t.Helper()

	// Reset memorystore to ensure test isolation from previous tests
	memorystore.DefaultStore.Reset()
	t.Logf("Reset memorystore before test setup for isolation")

	// First set up the base E2E environment
	baseEnv := SetupE2E(t)

	// Get LocalStack connection details
	localStackEndpoint := infra.NewLocalStackConnect(t)

	// Set up repositories needed for source loading
	sourceRepo := sources.New(log.NewLogger(io.Discard), baseEnv.App.DB)
	endpointRepo := postgres.NewEndpointRepo(baseEnv.App.DB)
	projectRepo := projects.New(baseEnv.App.Logger, baseEnv.App.DB)

	// Create the source loader and table for pubsub ingest
	lo := baseEnv.App.Logger.(*log.Logger)
	sourceLoader := pubsub.NewSourceLoader(endpointRepo, sourceRepo, projectRepo, lo)
	sourceTable := memorystore.NewTable(memorystore.OptionSyncer(sourceLoader))

	// Register the sources table with the memory store
	err := memorystore.DefaultStore.Register("sources", sourceTable)
	if err != nil {
		t.Logf("Source table registration: %v", err)
	}

	// Create the pubsub ingest component
	ingestComponent, err := pubsub.NewIngest(
		baseEnv.ctx,
		sourceTable,
		baseEnv.App.Queue,
		baseEnv.App.Logger,
		baseEnv.App.Rate,
		baseEnv.App.Licenser,
		"test-instance",
		endpointRepo,
	)
	require.NoError(t, err)

	// Start the ingest component in a goroutine
	_, cancelIngest := context.WithCancel(baseEnv.ctx)
	t.Logf("Starting ingest component in goroutine")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("PANIC in ingest component: %v", r)
			}
		}()
		t.Logf("Ingest goroutine started, calling Run()")
		ingestComponent.Run()
		t.Logf("Ingest Run() returned")
	}()

	// Give the ingest component time to start and sync the sources
	t.Logf("Waiting for ingest component to start...")
	time.Sleep(1 * time.Second)
	t.Logf("Ingest component should be running now")

	// Add cleanup for ingest component
	t.Cleanup(func() {
		t.Logf("Stopping SQS ingest component for test: %s", t.Name())
		cancelIngest()
		time.Sleep(200 * time.Millisecond)
	})

	sqsEnv := &E2ETestEnvWithSQS{
		E2ETestEnv:         baseEnv,
		LocalStackEndpoint: localStackEndpoint,
		Ingest:             ingestComponent,
		cancelIngest:       cancelIngest,
		sourceTable:        sourceTable,
		sourceContext:      context.Background(),
	}

	t.Logf("Test environment ready: %s (URL: %s, project: %s)", t.Name(), baseEnv.ServerURL, baseEnv.Project.UID)

	return sqsEnv
}

// SyncSources forces an immediate sync of the source table
func (env *E2ETestEnvWithSQS) SyncSources(t *testing.T) {
	t.Helper()
	err := env.sourceTable.Sync(env.sourceContext)
	if err != nil {
		t.Logf("Warning: failed to sync sources: %v", err)
	}
	// Give the ingest component time to process the new sources
	time.Sleep(1 * time.Second)
}

// SyncSubscriptions forces an immediate sync of the worker's subscription loader
func (env *E2ETestEnvWithSQS) SyncSubscriptions(t *testing.T) {
	t.Helper()

	// Access the worker's subscription loader from the App
	subLoader, ok := env.App.SubscriptionLoader.(*loader.SubscriptionLoader)
	if !ok {
		t.Fatal("SubscriptionLoader is not of expected type")
	}

	subTable, ok := env.App.SubscriptionTable.(*memorystore.Table)
	if !ok {
		t.Fatal("SubscriptionTable is not of expected type")
	}

	// Force an immediate sync of the worker's subscription loader
	err := subLoader.SyncChanges(env.ctx, subTable)
	if err != nil {
		t.Logf("Warning: failed to sync subscriptions: %v", err)
	}
	t.Logf("Forced worker subscription loader sync completed")

	// Give a small delay for the changes to propagate
	time.Sleep(500 * time.Millisecond)
}

// E2ETestEnvWithKafka extends E2ETestEnv with Kafka-specific resources
type E2ETestEnvWithKafka struct {
	*E2ETestEnv
	KafkaBroker   string
	Ingest        *pubsub.Ingest
	cancelIngest  context.CancelFunc
	sourceTable   *memorystore.Table
	sourceContext context.Context
}

// SetupE2EWithKafka sets up an E2E test environment with Kafka pubsub infrastructure
func SetupE2EWithKafka(t *testing.T) *E2ETestEnvWithKafka {
	t.Helper()

	// Reset memorystore to ensure test isolation from previous tests
	memorystore.DefaultStore.Reset()
	t.Logf("Reset memorystore before test setup for isolation")

	// First set up the base E2E environment
	baseEnv := SetupE2E(t)

	// Get Kafka broker connection details
	kafkaBroker := infra.NewKafkaConnect(t)

	// Set up repositories needed for source loading
	sourceRepo := sources.New(log.NewLogger(io.Discard), baseEnv.App.DB)
	endpointRepo := postgres.NewEndpointRepo(baseEnv.App.DB)
	projectRepo := projects.New(baseEnv.App.Logger, baseEnv.App.DB)

	// Create the source loader and table for pubsub ingest
	lo := baseEnv.App.Logger.(*log.Logger)
	sourceLoader := pubsub.NewSourceLoader(endpointRepo, sourceRepo, projectRepo, lo)
	sourceTable := memorystore.NewTable(memorystore.OptionSyncer(sourceLoader))

	// Register the sources table with the memory store
	err := memorystore.DefaultStore.Register("sources", sourceTable)
	if err != nil {
		t.Logf("Source table registration: %v", err)
	}

	// Create the pubsub ingest component
	ingestComponent, err := pubsub.NewIngest(
		baseEnv.ctx,
		sourceTable,
		baseEnv.App.Queue,
		baseEnv.App.Logger,
		baseEnv.App.Rate,
		baseEnv.App.Licenser,
		"test-instance",
		endpointRepo,
	)
	require.NoError(t, err)

	// Start the ingest component in a goroutine
	_, cancelIngest := context.WithCancel(baseEnv.ctx)
	t.Logf("Starting ingest component in goroutine")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("PANIC in ingest component: %v", r)
			}
		}()
		t.Logf("Ingest goroutine started, calling Run()")
		ingestComponent.Run()
		t.Logf("Ingest Run() returned")
	}()

	// Give the ingest component time to start and sync the sources
	t.Logf("Waiting for ingest component to start...")
	time.Sleep(1 * time.Second)
	t.Logf("Ingest component should be running now")

	// Add cleanup for ingest component
	t.Cleanup(func() {
		t.Logf("Stopping Kafka ingest component for test: %s", t.Name())
		cancelIngest()
		time.Sleep(200 * time.Millisecond)
	})

	kafkaEnv := &E2ETestEnvWithKafka{
		E2ETestEnv:    baseEnv,
		KafkaBroker:   kafkaBroker,
		Ingest:        ingestComponent,
		cancelIngest:  cancelIngest,
		sourceTable:   sourceTable,
		sourceContext: context.Background(),
	}

	t.Logf("Test environment ready: %s (URL: %s, project: %s)", t.Name(), baseEnv.ServerURL, baseEnv.Project.UID)

	return kafkaEnv
}

// SyncSources forces an immediate sync of the source table
func (env *E2ETestEnvWithKafka) SyncSources(t *testing.T) {
	t.Helper()
	err := env.sourceTable.Sync(env.sourceContext)
	if err != nil {
		t.Logf("Warning: failed to sync sources: %v", err)
	}
	// Give the ingest component time to process the new sources
	time.Sleep(1 * time.Second)
}

// SyncSubscriptions forces an immediate sync of the worker's subscription loader
func (env *E2ETestEnvWithKafka) SyncSubscriptions(t *testing.T) {
	t.Helper()

	// Access the worker's subscription loader from the App
	subLoader, ok := env.App.SubscriptionLoader.(*loader.SubscriptionLoader)
	if !ok {
		t.Fatal("SubscriptionLoader is not of expected type")
	}

	subTable, ok := env.App.SubscriptionTable.(*memorystore.Table)
	if !ok {
		t.Fatal("SubscriptionTable is not of expected type")
	}

	// Force an immediate sync of the worker's subscription loader
	err := subLoader.SyncChanges(env.ctx, subTable)
	if err != nil {
		t.Logf("Warning: failed to sync subscriptions: %v", err)
	}
	t.Logf("Forced worker subscription loader sync completed")

	// Give a small delay for the changes to propagate
	time.Sleep(500 * time.Millisecond)
}

// Google Pub/Sub Test Setup

type E2ETestEnvWithGooglePubSub struct {
	*E2ETestEnv
	PubSubEmulatorHost string
	ProjectID          string
	Ingest             *pubsub.Ingest
	cancelIngest       context.CancelFunc
	sourceTable        *memorystore.Table
	sourceContext      context.Context
}

// SetupE2EWithGooglePubSub sets up an E2E test environment with Google Pub/Sub infrastructure
func SetupE2EWithGooglePubSub(t *testing.T) *E2ETestEnvWithGooglePubSub {
	t.Helper()

	// Reset memorystore to ensure test isolation from previous tests
	memorystore.DefaultStore.Reset()
	t.Logf("Reset memorystore before test setup for isolation")

	// Get Pub/Sub emulator connection details
	// Note: PUBSUB_EMULATOR_HOST is set globally in TestMain
	emulatorHost := infra.NewPubSubEmulatorHost(t)
	projectID := "convoy-test-project"

	// Set up the base E2E environment (which starts the worker)
	baseEnv := SetupE2E(t)

	// Set up repositories needed for source loading
	sourceRepo := sources.New(log.NewLogger(io.Discard), baseEnv.App.DB)
	endpointRepo := postgres.NewEndpointRepo(baseEnv.App.DB)
	projectRepo := projects.New(baseEnv.App.Logger, baseEnv.App.DB)

	// Create the source loader and table for pubsub ingest
	lo := baseEnv.App.Logger.(*log.Logger)
	sourceLoader := pubsub.NewSourceLoader(endpointRepo, sourceRepo, projectRepo, lo)
	sourceTable := memorystore.NewTable(memorystore.OptionSyncer(sourceLoader))

	// Register the sources table with the memory store
	err := memorystore.DefaultStore.Register("sources", sourceTable)
	if err != nil {
		t.Logf("Source table registration: %v", err)
	}

	// Create the pubsub ingest component
	ingestComponent, err := pubsub.NewIngest(
		baseEnv.ctx,
		sourceTable,
		baseEnv.App.Queue,
		baseEnv.App.Logger,
		baseEnv.App.Rate,
		baseEnv.App.Licenser,
		"test-instance",
		endpointRepo,
	)
	require.NoError(t, err)

	// Start the ingest component in a goroutine
	_, cancelIngest := context.WithCancel(baseEnv.ctx)
	t.Logf("Starting ingest component in goroutine")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("PANIC in ingest component: %v", r)
			}
		}()
		t.Logf("Ingest goroutine started, calling Run()")
		ingestComponent.Run()
		t.Logf("Ingest Run() returned")
	}()

	// Register cleanup
	t.Cleanup(func() {
		t.Log("Stopping Google Pub/Sub ingest component for test:", t.Name())
		cancelIngest()
	})

	return &E2ETestEnvWithGooglePubSub{
		E2ETestEnv:         baseEnv,
		PubSubEmulatorHost: emulatorHost,
		ProjectID:          projectID,
		Ingest:             ingestComponent,
		cancelIngest:       cancelIngest,
		sourceTable:        sourceTable,
		sourceContext:      context.Background(),
	}
}

// SyncSources triggers a manual sync of sources into the memorystore table
func (env *E2ETestEnvWithGooglePubSub) SyncSources(t *testing.T) {
	t.Helper()
	t.Log("Manually syncing sources to memorystore...")
	err := env.sourceTable.Sync(env.sourceContext)
	require.NoError(t, err)
	t.Log("Source sync complete")
}

// SyncSubscriptions forces an immediate sync of the worker's subscription loader
func (env *E2ETestEnvWithGooglePubSub) SyncSubscriptions(t *testing.T) {
	t.Helper()

	// Access the worker's subscription loader from the App
	subLoader, ok := env.App.SubscriptionLoader.(*loader.SubscriptionLoader)
	if !ok {
		t.Fatal("SubscriptionLoader is not of expected type")
	}

	subTable, ok := env.App.SubscriptionTable.(*memorystore.Table)
	if !ok {
		t.Fatal("SubscriptionTable is not of expected type")
	}

	// Force an immediate sync of the worker's subscription loader
	err := subTable.Sync(env.ctx)
	if err != nil {
		t.Logf("Warning: failed to sync subscriptions: %v", err)
	}

	// Give the worker time to pick up the new subscriptions
	time.Sleep(1 * time.Second)

	t.Logf("Synced subscriptions: loader=%p, table=%p", subLoader, subTable)
}
