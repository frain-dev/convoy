package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"

	"github.com/frain-dev/convoy"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type Scheduler interface {
	Start()
	RegisterTask(cronSpec string, queueName convoy.QueueName, taskName convoy.TaskName)
	Stop()
}

type redisScheduler struct {
	log   log.Logger
	inner *asynq.Scheduler
}

type postgresScheduler struct {
	log   log.Logger
	queue queue.Queuer
	cron  *cron.Cron
	// registry is where entries are published so a process that is not running
	// the scheduler can still show what is scheduled. Nil when the driver does
	// not keep one, in which case registration is memory-only as before.
	registry     queue.SchedulerRegistry
	registeredID []string
}

func NewRedisScheduler(opts queue.QueueOptions, logger log.Logger) (Scheduler, error) {
	var redisConnOpt asynq.RedisConnOpt
	if opts.RedisFailoverOpt != nil {
		redisConnOpt = *opts.RedisFailoverOpt
	} else if opts.RedisClient != nil {
		redisConnOpt = opts.RedisClient
	} else {
		return nil, fmt.Errorf("redis scheduler connection is required")
	}
	scheduler := asynq.NewScheduler(redisConnOpt, &asynq.SchedulerOpts{
		Logger: logger,
	})

	return &redisScheduler{
		log:   logger,
		inner: scheduler,
	}, nil
}

func NewPostgresScheduler(q queue.Queuer, logger log.Logger) Scheduler {
	registry, _ := q.(queue.SchedulerRegistry)
	// UTC matches the asynq scheduler default, so a cron spec means the same
	// wall clock time under both providers regardless of the host timezone,
	// and no tick is skipped or repeated across a DST transition.
	return &postgresScheduler{
		log:      logger,
		queue:    q,
		cron:     cron.New(cron.WithLocation(time.UTC)),
		registry: registry,
	}
}

func (s *redisScheduler) Start() {
	if err := s.inner.Start(); err != nil {
		s.log.Fatal("Could not start scheduler", "error", err)
	}
}

func (s *redisScheduler) RegisterTask(cronSpec string, queueName convoy.QueueName, taskName convoy.TaskName) {
	task := asynq.NewTask(string(taskName), nil)
	id, err := s.inner.Register(cronSpec, task, asynq.Queue(string(queueName)))
	if err != nil {
		s.log.Fatalf("Failed to register %s scheduler task: %v", taskName, err)
	}
	s.log.Infof("Registered task %v with id %v", taskName, id)
}

func (s *redisScheduler) Stop() {
	s.inner.Shutdown()
}

func (s *postgresScheduler) Start() {
	s.cron.Start()
	s.prune()
}

func (s *postgresScheduler) RegisterTask(cronSpec string, queueName convoy.QueueName, taskName convoy.TaskName) {
	// The spec is parsed a second time so the next firing can be published
	// without waiting for the cron loop to compute it, which it only does once
	// it is running. AddFunc below uses the same standard parser, so a spec
	// that parses here is the one that will be scheduled.
	schedule, err := cron.ParseStandard(cronSpec)
	if err != nil {
		s.log.Fatalf("Failed to parse %s scheduler spec: %v", taskName, err)
	}

	id, err := s.cron.AddFunc(cronSpec, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// The tick-scoped ID makes cron enqueue idempotent across server
		// replicas while allowing the next scheduled tick to run. Failure
		// policy: a tick missed while this process was down is not replayed,
		// matching the asynq scheduler, which also has no catch-up.
		firedAt := time.Now()
		jobID := queue.CronJobID(taskName, firedAt)
		if err := s.queue.Write(ctx, taskName, queueName, &queue.Job{ID: jobID}); err != nil {
			s.log.Error("postgres scheduler enqueue failed", "task", taskName, "error", err)
		}
		s.record(ctx, cronSpec, queueName, taskName, schedule, &firedAt)
	})
	if err != nil {
		s.log.Fatalf("Failed to register %s scheduler task: %v", taskName, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), schedulerRegistryTimeout)
	defer cancel()
	s.record(ctx, cronSpec, queueName, taskName, schedule, nil)
	s.registeredID = append(s.registeredID, string(taskName))

	s.log.Infof("Registered task %v with id %v", taskName, id)
}

// schedulerRegistryTimeout bounds a registry write. It is short because the
// registry only feeds the monitoring page: a slow write must not hold up the
// enqueue it follows, which is the part that actually moves work.
const schedulerRegistryTimeout = 5 * time.Second

// record publishes what this process registered. Failure policy: log and carry
// on. The registry is a view of the schedule, not the schedule itself, so a
// failed write leaves the monitor showing a stale next run rather than stopping
// a task from firing.
//
// The entry id is the task name, which every replica derives identically, so
// replicas overwrite one row each instead of accumulating one row per process.
func (s *postgresScheduler) record(ctx context.Context, spec string, queueName convoy.QueueName, taskName convoy.TaskName, schedule cron.Schedule, firedAt *time.Time) {
	if s.registry == nil {
		return
	}

	next := schedule.Next(time.Now())
	err := s.registry.RecordSchedulerEntry(ctx, queue.SchedulerEntry{
		ID:       string(taskName),
		Spec:     spec,
		TaskName: string(taskName),
		Queue:    string(queueName),
		NextRun:  &next,
		PrevRun:  firedAt,
	})
	if err != nil {
		s.log.Error("postgres scheduler registry write failed", "task", taskName, "error", err)
	}
}

// prune drops entries for tasks this build no longer registers, so the monitor
// does not advertise a schedule nothing will fire.
func (s *postgresScheduler) prune() {
	if s.registry == nil || len(s.registeredID) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), schedulerRegistryTimeout)
	defer cancel()
	if err := s.registry.PruneSchedulerEntries(ctx, s.registeredID); err != nil {
		s.log.Error("postgres scheduler registry prune failed", "error", err)
	}
}

func (s *postgresScheduler) Stop() {
	<-s.cron.Stop().Done()
}
