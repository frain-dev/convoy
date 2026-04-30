package queue

import (
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
)

type Queuer interface {
	Write(convoy.TaskName, convoy.QueueName, *Job) error
	WriteWithoutTimeout(convoy.TaskName, convoy.QueueName, *Job) error
	Options() QueueOptions
}

type Job struct {
	ID      string        `json:"id"`
	Payload []byte        `json:"payload"`
	Delay   time.Duration `json:"delay"`

	// Headers is an optional W3C trace-context carrier populated by
	// tracectx.InjectIntoJob. Producers serialize it into the task envelope
	// so the consumer can rebuild a child context. Empty for non-traced
	// callers; consumers handle that case by starting a root span.
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
