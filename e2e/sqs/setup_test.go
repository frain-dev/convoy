package sqs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
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
	"github.com/frain-dev/convoy/internal/endpoints"
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
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/testenv"
)

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

type E2ETestEnvWithSQS struct {
	*E2ETestEnv
	LocalStackEndpoint string
	Ingest             *pubsub.Ingest
	cancelIngest       context.CancelFunc
	sourceTable        *memorystore.Table
	sourceContext      context.Context
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
		worker, err := cmdworker.NewWorker(workerCtx, app, cfg)
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

// SetupE2EWithSQS sets up an E2E test environment with SQS pubsub infrastructure
func SetupE2EWithSQS(t *testing.T) *E2ETestEnvWithSQS {
	t.Helper()

	// Reset memorystore to ensure test isolation from previous tests
	memorystore.DefaultStore.Reset()
	t.Logf("Reset memorystore before test setup for isolation")

	// First set up the base E2E environment
	baseEnv := SetupE2E(t)

	// Get LocalStack connection details
	localStackEndpoint := (*infra.NewLocalStackConnect)(t)

	// Set up repositories needed for source loading
	sourceRepo := sources.New(log.New("convoy", log.LevelError), baseEnv.App.DB)
	endpointRepo := endpoints.New(baseEnv.App.Logger, baseEnv.App.DB)
	projectRepo := projects.New(baseEnv.App.Logger, baseEnv.App.DB)

	// Create the source loader and table for pubsub ingest
	lo := baseEnv.App.Logger
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

// waitForServer waits for the HTTP server to become available
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
