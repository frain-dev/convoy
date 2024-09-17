package circuit_breaker

import (
	"github.com/frain-dev/convoy/pkg/msgpack"
	"time"
)

// CircuitBreaker represents a circuit breaker
type CircuitBreaker struct {
	// Circuit breaker key
	Key string `json:"key"`
	// Circuit breaker state
	State State `json:"state"`
	// Number of requests in the observability window
	Requests uint64 `json:"requests"`
	// Percentage of failures in the observability window
	FailureRate float64 `json:"failure_rate"`
	// Time after which the circuit breaker will reset
	WillResetAt time.Time `json:"will_reset_at"`
	// Number of failed requests in the observability window
	TotalFailures uint64 `json:"total_failures"`
	// Number of successful requests in the observability window
	TotalSuccesses uint64 `json:"total_successes"`
	// Number of consecutive circuit breaker trips
	ConsecutiveFailures uint64 `json:"consecutive_failures"`
}

func (b *CircuitBreaker) String() (s string, err error) {
	bytes, err := msgpack.EncodeMsgPack(b)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func (b *CircuitBreaker) tripCircuitBreaker(resetTime time.Time) {
	b.State = StateOpen
	b.WillResetAt = resetTime
	b.ConsecutiveFailures++
}

func (b *CircuitBreaker) toHalfOpen() {
	b.State = StateHalfOpen
}

func (b *CircuitBreaker) resetCircuitBreaker() {
	b.State = StateClosed
	b.ConsecutiveFailures = 0
}
