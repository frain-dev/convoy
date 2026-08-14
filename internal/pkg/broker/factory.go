package broker

import (
	"fmt"
	"strconv"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	pgcache "github.com/frain-dev/convoy/cache/postgres"
	rcache "github.com/frain-dev/convoy/cache/redis"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/batch_tracker"
	"github.com/frain-dev/convoy/internal/pkg/dynamiceventack"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	pglimiter "github.com/frain-dev/convoy/internal/pkg/limiter/postgres"
	rlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/redis"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/clock"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	pgqueue "github.com/frain-dev/convoy/queue/postgres"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
)

type Dependencies struct {
	Queue               queue.Queuer
	QueueMonitor        queue.Monitor
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
	return build(cfg, db, logger)
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
	c := pgcache.New(db)

	lockDB, err := openJobLockDB(cfg.Database)
	if err != nil {
		return nil, err
	}

	return &Dependencies{
		Queue:               q,
		QueueMonitor:        q,
		Cache:               c,
		RateLimiter:         pglimiter.New(db),
		CircuitBreakerStore: circuit_breaker.NewPostgresStore(db),
		JobLocker:           newPostgresJobLockerFromDB(lockDB, logger),
		Acker:               dynamiceventack.NewCacheAcker(c),
		TrialEvents:         license.NewPostgresTrialEventLimiter(db, logger),
		ConsumerBackend:     worker.NewPostgresConsumerBackend(q),
		Scheduler:           worker.NewPostgresScheduler(q, logger),
		TaskErrors:          q,
		ResendClaims:        services.NewPostgresResendClaimStore(db),
		BatchTracker:        batch_tracker.NewPostgresTracker(db),
	}, nil
}

func newRedis(cfg config.Configuration, _ *sqlx.DB, logger log.Logger) (*Dependencies, error) {
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
		db, _ := strconv.Atoi(cfg.Redis.Database)
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
	c := rcache.NewRedisCacheFromClient(rd.Client())
	rateLimiter := rlimiter.NewLimiterFromRedisClient(rd.Client())
	scheduler, err := worker.NewRedisScheduler(opts, logger)
	if err != nil {
		return nil, err
	}

	return &Dependencies{
		Queue:               q,
		QueueMonitor:        q,
		Cache:               c,
		RateLimiter:         rateLimiter,
		CircuitBreakerStore: circuit_breaker.NewRedisStore(rd.Client(), clock.NewRealClock()),
		JobLocker:           newRedisJobLocker(rd.Client(), logger),
		Acker:               dynamiceventack.NewRedisAcker(rd.Client()),
		TrialEvents:         license.NewTrialEventLimiter(rd.Client(), logger),
		ConsumerBackend:     worker.NewRedisConsumerBackend(opts),
		Scheduler:           scheduler,
		TaskErrors:          q,
		ResendClaims:        services.NewRedisResendClaimStore(rd.Client()),
		BatchTracker:        batch_tracker.NewBatchTracker(rd.Client()),
	}, nil
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
	}, nil
}
