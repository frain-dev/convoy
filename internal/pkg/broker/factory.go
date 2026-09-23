package broker

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	pgcache "github.com/frain-dev/convoy/cache/postgres"
	rcache "github.com/frain-dev/convoy/cache/redis"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/batch_tracker"
	"github.com/frain-dev/convoy/internal/pkg/dynamiceventack"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	pglimiter "github.com/frain-dev/convoy/internal/pkg/limiter/postgres"
	rlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/redis"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/clock"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/drain"
	"github.com/frain-dev/convoy/queue/inventory"
	pgqueue "github.com/frain-dev/convoy/queue/postgres"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
)

type Dependencies struct {
	Source       *Dependencies
	Drain        *drain.Controller
	Queue        queue.Queuer
	WorkerQueue  queue.Queuer
	MetricsQueue queue.Queuer
	Admission    *drain.Gate
	Fence        drain.Fence
	// QueueMonitor is asynqmon, which only the redis broker has. The postgres
	// broker leaves it nil: its monitoring surface is QueueInspector, which
	// both brokers implement and the dashboard renders natively.
	QueueMonitor        queue.Monitor
	QueueInspector      queue.Inspector
	Cache               cache.AuthoritativeCache
	RateLimiter         limiter.RateLimiter
	CircuitBreakerStore circuit_breaker.CircuitBreakerStore
	JobLocker           task.JobLocker
	Acker               dynamiceventack.Acker
	TrialEvents         *license.TrialEventLimiter
	ConsumerBackend     worker.ConsumerBackend
	Scheduler           worker.Scheduler
	TaskErrors          task.TaskErrorReader
	ResendClaims        services.ResendClaimStore
	BatchTracker        batch_tracker.Tracker

	closers []io.Closer
}

// Close releases what the broker opened for itself: the dedicated advisory lock
// pool in postgres mode, the client in redis mode. The application database is
// passed in rather than opened here, so it stays with its owner. A long-lived
// process can skip this; anything that builds a broker per unit of work, tests
// included, must call it or leak a pool per build.
func (d *Dependencies) Close() error {
	var firstErr error
	for _, c := range d.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type constructor func(config.Configuration, *sqlx.DB, log.Logger) (*Dependencies, error)

var constructors = map[config.QueueProvider]constructor{
	config.PostgresQueueProvider: newPostgres,
	config.RedisQueueProvider:    newRedis,
}

var newRedisClient = rdb.NewClientFromRedisConfig

func New(cfg config.Configuration, db *sqlx.DB, logger log.Logger) (*Dependencies, error) {
	build, ok := constructors[cfg.QueueProvider]
	if !ok {
		return nil, fmt.Errorf("unsupported broker provider %q", cfg.QueueProvider)
	}
	if cfg.QueueProvider == config.PostgresQueueProvider {
		if !fflag.NewFFlag(cfg.EnableFeatureFlag).CanAccessFeature(fflag.PostgresQueue) {
			return nil, fflag.ErrPostgresQueueNotEnabled
		}
	}
	if err := cfg.ValidateQueueDrain(); err != nil {
		return nil, err
	}
	active, err := build(cfg, db, logger)
	if err != nil {
		return nil, err
	}
	if cfg.Queue.Drain.Enabled && cfg.PreviousQueueProvider != "" {
		sourceCfg := cfg
		sourceCfg.QueueProvider = cfg.PreviousQueueProvider
		sourceCfg.PreviousQueueProvider = ""
		sourceBuild, exists := constructors[sourceCfg.QueueProvider]
		if !exists {
			_ = active.Close()
			return nil, fmt.Errorf("unsupported previous queue provider")
		}
		source, buildErr := sourceBuild(sourceCfg, db, logger)
		if buildErr != nil {
			_ = active.Close()
			return nil, buildErr
		}
		source.Drain.Previous = true
		source.Admission.Previous = true
		source.Drain.Target.ConfigurationRevision = drain.ConfigurationRevision(cfg)
		active.Source = source
		active.Drain.Participants = append(active.Drain.Participants, source.Drain.Participants...)
		active.closers = append(active.closers, source)
		active.Drain.Target.OtherStoreID = source.Drain.Target.StoreID
		active.Drain.Fence = drain.StoreGroup{Active: active.Fence, Previous: source.Fence}
		active.Drain.Reader = drain.StoreReaders{Active: active.Drain.Reader, Previous: source.Drain.Reader}
	}
	if active.Drain != nil {
		executors := map[string]queue.Queuer{active.Drain.Target.StoreID: active.WorkerQueue}
		if active.Source != nil {
			executors[active.Source.Drain.Target.StoreID] = active.Source.WorkerQueue
		}
		active.Queue = drain.RouteDescendants(active.Queue, cfg.QueueStoreScope, executors)
	}
	return active, nil
}

// AllowPostgresQueue is the license half of the postgres provider gate.
// The feature flag is checked in New because config is available then; the
// licenser is created later in PreRun, so this runs immediately after it.
// Redis is unaffected. Missing entitlement is refuse-to-boot, not a redis fallback.
func AllowPostgresQueue(cfg config.Configuration, licenser license.Licenser) error {
	if cfg.QueueProvider != config.PostgresQueueProvider {
		return nil
	}
	if licenser == nil || !licenser.PostgresQueue() {
		return fmt.Errorf("postgres queue requires the postgres_queue license entitlement")
	}
	return nil
}

func newPostgres(cfg config.Configuration, db *sqlx.DB, logger log.Logger) (*Dependencies, error) {
	if db == nil {
		return nil, fmt.Errorf("postgres broker requires a database")
	}
	opts, err := queueOptions(cfg)
	if err != nil {
		return nil, err
	}
	opts.DB = db
	q, err := pgqueue.NewQueue(opts)
	if err != nil {
		return nil, err
	}
	workerQueue := q
	var gate *drain.Gate
	var fence drain.Fence
	var executorDB *sqlx.DB
	if cfg.Queue.Drain.Enabled {
		executorDB, err = drain.PostgresExecutorDB(cfg)
		if err != nil {
			_ = q.Close()
			return nil, err
		}
		executorOpts := opts
		executorOpts.DB = executorDB
		workerQueue, err = pgqueue.NewQueue(executorOpts)
		if err != nil {
			_ = q.Close()
			_ = executorDB.Close()
			return nil, err
		}
		gate = &drain.Gate{Repository: drain.NewRepository(db), Scope: cfg.QueueStoreScope, StoreID: inventory.StoreID(cfg, "postgres")}
		fence = drain.PostgresFence{DB: db, Scope: gate.Scope, StoreID: gate.StoreID, ExecutorRole: cfg.Queue.Drain.PostgresExecutorUsername}
		if err = fence.Bind(context.Background()); err != nil {
			_ = q.Close()
			_ = workerQueue.Close()
			_ = executorDB.Close()
			return nil, err
		}
	}
	c := pgcache.NewWithLocalReads(db, cfg.Cache.Postgres.LocalReadTTL(), cfg.Cache.Postgres.LocalReadSize)

	lockDB, err := openJobLockDB(cfg.Database)
	if err != nil {
		return nil, err
	}

	dependencies := &Dependencies{
		Queue:               q,
		MetricsQueue:        workerQueue,
		QueueInspector:      workerQueue,
		Cache:               c,
		RateLimiter:         pglimiter.New(db),
		CircuitBreakerStore: circuit_breaker.NewPostgresStore(db),
		JobLocker:           newPostgresJobLockerWithLimit(lockDB, logger, jobLockMaxConns),
		Acker:               dynamiceventack.NewCacheAcker(c),
		TrialEvents:         license.NewPostgresTrialEventLimiter(db, logger),
		ConsumerBackend:     worker.NewPostgresConsumerBackend(workerQueue),
		Scheduler:           worker.NewPostgresScheduler(q, logger),
		TaskErrors:          q,
		ResendClaims:        services.NewPostgresResendClaimStore(db),
		BatchTracker:        batch_tracker.NewPostgresTracker(db),
		// q first: its batchers must stop before the lock pool goes.
		closers: []io.Closer{q, lockDB},
	}
	if gate != nil {
		dependencies.Admission, dependencies.Fence = gate, fence
		dependencies.Drain = &drain.Controller{Repository: gate.Repository, Target: drain.Target{Scope: gate.Scope, StoreID: gate.StoreID, Provider: string(cfg.QueueProvider), ConfigurationRevision: drain.ConfigurationRevision(cfg)}, Fence: fence, Reader: inventory.Postgres{DB: db}}
		dependencies.Drain.Participants = []drain.Participant{{StoreID: gate.StoreID, Reader: dependencies.Drain.Reader, Inspector: dependencies.QueueInspector}}
		dependencies.Queue = gate.GuardQueue(q)
		dependencies.WorkerQueue = gate.GuardQueue(workerQueue)
		dependencies.closers = append(dependencies.closers, workerQueue, executorDB)
	}
	return dependencies, nil
}

func newRedis(cfg config.Configuration, db *sqlx.DB, logger log.Logger) (*Dependencies, error) {
	rd, err := newRedisClient(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("connect redis broker: %w", err)
	}
	opts, err := queueOptions(cfg)
	if err != nil {
		return nil, err
	}
	opts.RedisClient = rd
	opts.RedisAddress = cfg.Redis.BuildDsn()
	if cfg.Redis.IsSentinel() {
		// An unset database means 0, the same reading BuildDsn takes. Only a
		// non-empty value that will not parse is a misconfiguration.
		var db int
		if cfg.Redis.Database != "" {
			db, err = strconv.Atoi(cfg.Redis.Database)
			if err != nil {
				return nil, fmt.Errorf("parse redis database %q: %w", cfg.Redis.Database, err)
			}
		}
		opts.RedisFailoverOpt = &asynq.RedisFailoverClientOpt{
			MasterName:       cfg.Redis.MasterName,
			SentinelAddrs:    cfg.Redis.SentinelAddresses(),
			Username:         cfg.Redis.Username,
			Password:         cfg.Redis.Password,
			SentinelPassword: cfg.Redis.SentinelPassword,
			DB:               db,
		}
	}
	q := redisqueue.NewQueue(opts)
	runtimeRD := rd
	runtimeOpts := opts
	workerQueue := q
	var gate *drain.Gate
	var fence drain.Fence
	var controller *rdb.Redis
	if cfg.Queue.Drain.Enabled {
		if db == nil {
			_ = rd.Client().Close()
			return nil, fmt.Errorf("queue drain requires a database")
		}
		executorCfg, credentialErr := drain.RedisCredentials(cfg.Redis, cfg.Queue.Drain.RedisExecutorUsername, cfg.Queue.Drain.RedisExecutorPassword)
		if credentialErr != nil {
			_ = rd.Client().Close()
			return nil, credentialErr
		}
		runtimeRD, err = newRedisClient(executorCfg)
		if err != nil {
			_ = rd.Client().Close()
			return nil, fmt.Errorf("connect queue executor: %w", err)
		}
		runtimeOpts.RedisClient = runtimeRD
		runtimeOpts.RedisAddress = executorCfg.BuildDsn()
		workerQueue = redisqueue.NewQueue(runtimeOpts)
		controllerCfg, credentialErr := drain.RedisCredentials(cfg.Redis, cfg.Queue.Drain.RedisControllerUsername, cfg.Queue.Drain.RedisControllerPassword)
		if credentialErr != nil {
			_ = rd.Client().Close()
			_ = runtimeRD.Client().Close()
			return nil, credentialErr
		}
		controller, err = newRedisClient(controllerCfg)
		if err != nil {
			_ = rd.Client().Close()
			_ = runtimeRD.Client().Close()
			return nil, fmt.Errorf("connect queue controller: %w", err)
		}
		admin, ok := controller.Client().(*redis.Client)
		if !ok {
			_ = rd.Client().Close()
			_ = runtimeRD.Client().Close()
			_ = controller.Client().Close()
			return nil, fmt.Errorf("queue fence requires standalone Redis")
		}
		producer, parseErr := redis.ParseURL(cfg.Redis.BuildDsn()[0])
		if parseErr != nil {
			_ = rd.Client().Close()
			_ = runtimeRD.Client().Close()
			_ = controller.Client().Close()
			return nil, fmt.Errorf("invalid queue producer endpoint")
		}
		if producer.Username == "" {
			producer.Username = "default"
		}
		gate = &drain.Gate{Repository: drain.NewRepository(db), Scope: cfg.QueueStoreScope, StoreID: inventory.StoreID(cfg, "redis")}
		fence = drain.RedisFence{Repository: gate.Repository, Admin: admin, Scope: gate.Scope, StoreID: gate.StoreID, ProducerUser: producer.Username, ExecutorUser: cfg.Queue.Drain.RedisExecutorUsername, ControllerUser: cfg.Queue.Drain.RedisControllerUsername}
		if err = fence.Bind(context.Background()); err != nil {
			_ = rd.Client().Close()
			_ = runtimeRD.Client().Close()
			_ = controller.Client().Close()
			return nil, err
		}
	}
	c := rcache.NewRedisCacheFromClient(runtimeRD.Client())
	rateLimiter := rlimiter.NewLimiterFromRedisClient(runtimeRD.Client())
	scheduler, err := worker.NewRedisScheduler(opts, logger)
	if err != nil {
		return nil, err
	}

	dependencies := &Dependencies{
		Queue:               q,
		MetricsQueue:        workerQueue,
		QueueMonitor:        workerQueue,
		QueueInspector:      workerQueue,
		Cache:               c,
		RateLimiter:         rateLimiter,
		CircuitBreakerStore: circuit_breaker.NewRedisStore(runtimeRD.Client(), clock.NewRealClock()),
		JobLocker:           newRedisJobLocker(runtimeRD.Client(), logger),
		Acker:               dynamiceventack.NewRedisAcker(runtimeRD.Client()),
		TrialEvents:         license.NewTrialEventLimiter(runtimeRD.Client(), logger),
		ConsumerBackend:     worker.NewRedisConsumerBackend(runtimeOpts),
		Scheduler:           scheduler,
		TaskErrors:          workerQueue,
		ResendClaims:        services.NewRedisResendClaimStore(runtimeRD.Client()),
		BatchTracker:        batch_tracker.NewBatchTracker(runtimeRD.Client()),
		closers:             []io.Closer{runtimeRD.Client()},
	}
	if gate != nil {
		dependencies.Admission, dependencies.Fence = gate, fence
		readerCfg, _ := drain.RedisCredentials(cfg.Redis, cfg.Queue.Drain.RedisExecutorUsername, cfg.Queue.Drain.RedisExecutorPassword)
		dependencies.Drain = &drain.Controller{Repository: gate.Repository, Target: drain.Target{Scope: gate.Scope, StoreID: gate.StoreID, Provider: string(cfg.QueueProvider), ConfigurationRevision: drain.ConfigurationRevision(cfg)}, Fence: fence, Reader: inventory.RedisReader(readerCfg)}
		dependencies.Drain.Participants = []drain.Participant{{StoreID: gate.StoreID, Reader: dependencies.Drain.Reader, Inspector: dependencies.QueueInspector}}
		dependencies.Queue = gate.GuardQueue(q)
		dependencies.WorkerQueue = gate.GuardQueue(workerQueue)
		dependencies.closers = append(dependencies.closers, rd.Client(), controller.Client())
	}
	return dependencies, nil
}

func QueueNames(mode config.ExecutionMode) (map[string]int, error) {
	events := map[string]int{
		string(convoy.EventQueue):         5,
		string(convoy.CreateEventQueue):   5,
		string(convoy.EventWorkflowQueue): 5,
	}
	retry := map[string]int{
		string(convoy.RetryEventQueue):    7,
		string(convoy.ScheduleQueue):      1,
		string(convoy.DefaultQueue):       1,
		string(convoy.MetaEventQueue):     1,
		string(convoy.BatchRetryQueue):    5,
		string(convoy.EventWorkflowQueue): 4,
	}
	both := map[string]int{
		string(convoy.EventQueue):         4,
		string(convoy.CreateEventQueue):   4,
		string(convoy.EventWorkflowQueue): 3,
		string(convoy.RetryEventQueue):    1,
		string(convoy.ScheduleQueue):      1,
		string(convoy.DefaultQueue):       1,
		string(convoy.MetaEventQueue):     1,
		string(convoy.BatchRetryQueue):    1,
	}
	executionQueues := map[config.ExecutionMode]map[string]int{
		config.RetryExecutionMode:   retry,
		config.EventsExecutionMode:  events,
		config.DefaultExecutionMode: both,
	}
	names, ok := executionQueues[mode]
	if !ok {
		return nil, fmt.Errorf("unknown execution mode: %s", mode)
	}
	return names, nil
}

func queueOptions(cfg config.Configuration) (queue.QueueOptions, error) {
	names, err := QueueNames(cfg.WorkerExecutionMode)
	if err != nil {
		return queue.QueueOptions{}, err
	}
	return queue.QueueOptions{
		Names:             names,
		Type:              string(cfg.QueueProvider),
		PrometheusAddress: cfg.Prometheus.Dsn,
		PostgresTuning: queue.PostgresTuning{
			BatchSize:        cfg.Queue.Postgres.BatchSize,
			BatchWait:        cfg.Queue.Postgres.BatchWait(),
			WriteConcurrency: cfg.Queue.Postgres.WriteConcurrency,
			LeaseTimeout:     cfg.Queue.Postgres.LeaseTimeout(),
			ClaimBatchSize:   cfg.Queue.Postgres.ClaimBatchSize,
			PollIdle:         cfg.Queue.Postgres.PollIdle(),
		},
		PostgresConnString: cfg.Database.BuildDsn(),
	}, nil
}
