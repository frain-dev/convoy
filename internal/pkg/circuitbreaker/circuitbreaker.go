package circuitbreaker

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker/v2"
	"time"
)

type CircuitBreaker interface {
	Execute() error
}

func NewCircuitBreaker(name string, cfg config.RedisConfiguration) (CircuitBreaker, error) {
	r, err := rdb.NewClient(cfg.BuildDsn())
	if err != nil {
		return nil, err
	}
	return NewRedisCircuitBreaker(name, r.Client())
}

// State is a type that represents a state of the CircuitBreaker.
type State int

const prefix = "breaker"

type RedisCircuitBreaker struct {
	breaker       *gobreaker.CircuitBreaker[bool]
	resetDuration time.Duration
}

type BreakerEntry struct {
	Requests             uint32    `json:"requests"`
	TotalFailures        uint32    `json:"total_failures"`
	TotalSuccesses       uint32    `json:"total_successes"`
	LastTriggeredAt      time.Time `json:"last_triggered_at"`
	ConsecutiveFailures  uint32    `json:"consecutive_failures"`
	ConsecutiveSuccesses uint32    `json:"consecutive_successes"`
}

func NewRedisCircuitBreaker(name string, client redis.UniversalClient) (*RedisCircuitBreaker, error) {
	// build the redis key
	key := fmt.Sprintf("%s:%s", prefix, name)

	// load breaker state from redis into memory
	val := client.Get(context.Background(), key).Val()
	if val != redis.Nil.Error() {
		// init breaker here
	}

	// if state is nil create a new breaker
	b := gobreaker.NewCircuitBreaker[bool](gobreaker.Settings{
		Name:        name,
		MaxRequests: 0,
		Interval:    time.Second * 30,
		Timeout:     time.Second * 5,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 0
		},
	})

	return &RedisCircuitBreaker{
		breaker: b,
	}, nil
}

func (c *RedisCircuitBreaker) Execute() error {
	// run the circuit breaker's Execute func

	// get the state from redis again

	// update the state on redis

	return nil
}
