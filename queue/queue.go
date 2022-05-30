package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
)

type Queuer interface {
	NewQueue(opts QueueOptions) error
	Write(context.Context, string, string, *Job) error
	StartOne(context.Context, string) error
	StartAll(context.Context) error
	Update(context.Context, QueueOptions) error
	Stats(string) (*Stats, error)
	Delete(string) error
	Length(string) (int, error)
	StopOne(string) error
	StopAll() error
	Contains(string) bool
}

type Job struct {
	Err     error           `json:"err"`
	Payload json.RawMessage `json:"payload"`
	Delay   time.Duration   `json:"delay"`
}

type Stats struct {
	Name      string
	Processed int
	Retries   int
	Fails     int
}

type QueueOptions struct {
	Name        string
	Type        string
	Redis       *redis.Client
	Concurrency int
}
