package queue

import (
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/hibiken/asynq"
)

type Queuer interface {
	Write(convoy.TaskName, convoy.QueueName, *Job) error
	Options() QueueOptions
}

type Job struct {
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
	Delay   time.Duration   `json:"delay"`
}

type QueueOptions struct {
	Names             map[string]int
	Type              string
	Client            *asynq.Client
	RedisAddress      string
	PrometheusAddress string
}
