package circuit_breaker

import (
	"github.com/frain-dev/convoy/pkg/msgpack"
	"time"
)

// CircuitBreaker represents a circuit breaker
type CircuitBreaker struct {
	Key                 string    `json:"key"`
	State               State     `json:"state"`
	Requests            uint64    `json:"requests"`
	FailureRate         float64   `json:"failure_rate"`
	WillResetAt         time.Time `json:"will_reset_at"`
	TotalFailures       uint64    `json:"total_failures"`
	TotalSuccesses      uint64    `json:"total_successes"`
	ConsecutiveFailures uint64    `json:"consecutive_failures"`
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
