package queue

import (
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
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
	Names                 map[string]int
	Type                  string
	RedisClient           *rdb.Redis
	RedisClusterAddresses []string
	RedisAddress          string
	PrometheusAddress     string
}
