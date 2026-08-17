package queue

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
)

const (
	ProviderRedis    = "redis"
	ProviderPostgres = "postgres"
)

func PriorityCycle(weights map[string]int) []string {
	names := make([]string, 0, len(weights))
	for name := range weights {
		names = append(names, name)
	}
	sort.Strings(names)
	total := 0
	for _, name := range names {
		if weights[name] > 0 {
			total += weights[name]
		}
	}
	if total == 0 {
		return names
	}
	current := make(map[string]int, len(names))
	cycle := make([]string, 0, total)
	for range total {
		best := ""
		for _, name := range names {
			weight := weights[name]
			if weight <= 0 {
				continue
			}
			current[name] += weight
			if best == "" || current[name] > current[best] {
				best = name
			}
		}
		current[best] -= total
		cycle = append(cycle, best)
	}
	return cycle
}

// Queuer enqueues asynq tasks. The driver injects the active OTel trace
// context from ctx into the task's headers so worker spans become children
// of the producer's; callers don't need to do anything special.
type Queuer interface {
	Write(ctx context.Context, taskName convoy.TaskName, queueName convoy.QueueName, job *Job) error
	WriteWithoutTimeout(ctx context.Context, taskName convoy.TaskName, queueName convoy.QueueName, job *Job) error
	Options() QueueOptions
}

// Monitor is the queue dashboard (asynqmon or the postgres HTML/JSON UI).
type Monitor interface {
	Monitor() http.Handler
	MonitorWithRootPath(rootPath string) http.Handler
}

// Archiver deletes completed/archived jobs. Both drivers implement it.
type Archiver interface {
	DeleteArchived(ctx context.Context) error
}

var (
	// ErrTaskNotFound means no row carries that task ID. Failure policy: fail
	// closed. A lookup that errored is returned as itself, never reported as a
	// missing task.
	ErrTaskNotFound = errors.New("queue: task not found")

	// ErrTaskStatusConflict means the row exists but is not in the status the
	// requested action moves from.
	ErrTaskStatusConflict = errors.New("queue: task is not in the required status")

	// ErrCronTaskImmutable protects scheduler rows. A finished cron row is the
	// tombstone that keeps one tick from being enqueued twice across replicas,
	// so it is not an operator's to retry or archive.
	ErrCronTaskImmutable = errors.New("queue: scheduler rows cannot be retried or archived")

	// ErrUnknownTaskStatus rejects a status the running provider does not
	// serve, rather than returning an empty page that reads as "nothing queued".
	ErrUnknownTaskStatus = errors.New("queue: unknown task status")

	// ErrQueueRequired rejects a drill-down that names no queue.
	ErrQueueRequired = errors.New("queue: queue name is required")

	// ErrQueueNotFound means the broker holds no queue under that name.
	ErrQueueNotFound = errors.New("queue: queue not found")

	// ErrInvalidPage rejects a page outside the bounded drill-down window.
	ErrInvalidPage = errors.New("queue: page is out of range")
)

type Job struct {
	ID      string        `json:"id"`
	Payload []byte        `json:"payload"`
	Delay   time.Duration `json:"delay"`

	// MaxRetry, when set, caps how many times asynq will retry this task
	// before archiving it. It is used to sync asynq's per-task retry budget
	// with Convoy's configured retry limit so deliveries are not silently
	// retried up to asynq's default (25). Nil leaves the asynq default.
	MaxRetry *int `json:"-"`

	// Headers carries the W3C trace context. The Queuer driver fills this
	// from the active OTel span on the producer's ctx and feeds it into
	// asynq.NewTaskWithHeaders so it rides alongside the payload. Callers
	// rarely set it directly. Empty for untraced enqueues; the consumer
	// middleware starts a root span in that case.
	Headers map[string]string `json:"-"`
}

// ClaimedJob is a broker-neutral job claimed by a queue consumer.
type ClaimedJob struct {
	ID         string
	TaskName   string
	QueueName  string
	Payload    []byte
	Headers    map[string]string
	MaxRetry   int
	RetryCount int
}

type QueueOptions struct {
	Names             map[string]int
	Type              string
	RedisClient       *rdb.Redis
	RedisAddress      []string
	RedisFailoverOpt  *asynq.RedisFailoverClientOpt
	PrometheusAddress string
	// DB is required when Type is postgres. Redis queues ignore it.
	DB *sqlx.DB
	// PostgresTuning tunes the postgres write path. Redis queues ignore it,
	// and zero fields take the postgres driver's defaults.
	PostgresTuning PostgresTuning
	// PostgresConnString enables LISTEN/NOTIFY consumer wakeups when set.
	// Redis queues ignore it.
	PostgresConnString string
}

// PostgresTuning carries the postgres queue's write-path settings in
// provider-neutral form so the queue package does not depend on config.
type PostgresTuning struct {
	BatchSize        int
	BatchWait        time.Duration
	WriteConcurrency int
	LeaseTimeout     time.Duration
	// ClaimBatchSize is how many jobs one Claim round trip may take. Consumers
	// cap this at pool size so a small pool does not over-claim.
	ClaimBatchSize int
	// PollIdle is how long the consumer waits when Claim returns no work.
	PollIdle time.Duration
}

type JobId struct {
	ProjectID  string
	ResourceID string
}

func (j JobId) SingleJobId() string {
	return fmt.Sprintf("single:%s:%s", j.ProjectID, j.ResourceID)
}

func (j JobId) MetaJobId() string {
	return fmt.Sprintf("meta:%s:%s", j.ProjectID, j.ResourceID)
}

func (j JobId) DynamicJobId() string {
	return fmt.Sprintf("dynamic:%s:%s", j.ProjectID, j.ResourceID)
}

func (j JobId) BroadcastJobId() string {
	return fmt.Sprintf("broadcast:%s:%s", j.ProjectID, j.ResourceID)
}

func (j JobId) FanOutJobId() string {
	return fmt.Sprintf("fanout:%s:%s", j.ProjectID, j.ResourceID)
}

func (j JobId) ReplayJobId() string {
	return fmt.Sprintf("replay:%s:%s", j.ProjectID, j.ResourceID)
}

func (j JobId) MatchSubsJobId() string {
	return fmt.Sprintf("match_subs:%s:%s", j.ProjectID, j.ResourceID)
}

func (j JobId) OnboardJobId() string {
	return fmt.Sprintf("onboard:%s:%s", j.ProjectID, j.ResourceID)
}

// CronJobIDPrefix marks a job as a scheduler firing. Drivers that deduplicate
// cron ticks match on it, so the writer and the matcher share one constant.
const CronJobIDPrefix = "cron:"

// CronJobID is the deduplication key for one scheduler tick. Every replica
// that fires the same tick derives the same ID, so a driver with a unique
// job id enqueues the tick once no matter how many schedulers are running.
// The bucket is the UTC minute of the firing, which is also the granularity
// of a cron spec: a firing delayed past its own minute is a new tick.
func CronJobID(taskName convoy.TaskName, at time.Time) string {
	return fmt.Sprintf("%s%s:%d", CronJobIDPrefix, taskName, at.UTC().Truncate(time.Minute).Unix())
}
