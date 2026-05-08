package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
)

// Queuer enqueues asynq tasks. The driver injects the active OTel trace
// context from ctx into the task's headers so worker spans become children
// of the producer's; callers don't need to do anything special.
type Queuer interface {
	Write(ctx context.Context, taskName convoy.TaskName, queueName convoy.QueueName, job *Job) error
	WriteWithoutTimeout(ctx context.Context, taskName convoy.TaskName, queueName convoy.QueueName, job *Job) error
	Options() QueueOptions
}

type Job struct {
	ID      string        `json:"id"`
	Payload []byte        `json:"payload"`
	Delay   time.Duration `json:"delay"`

	// Headers carries the W3C trace context. The Queuer driver fills this
	// from the active OTel span on the producer's ctx and feeds it into
	// asynq.NewTaskWithHeaders so it rides alongside the payload. Callers
	// rarely set it directly. Empty for untraced enqueues; the consumer
	// middleware starts a root span in that case.
	Headers map[string]string `json:"-"`
}

type QueueOptions struct {
	Names             map[string]int
	Type              string
	RedisClient       *rdb.Redis
	RedisAddress      []string
	RedisFailoverOpt  *asynq.RedisFailoverClientOpt
	PrometheusAddress string
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
