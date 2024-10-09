package circuit_breaker

import (
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"time"
)

// CircuitBreaker represents a circuit breaker
type CircuitBreaker struct {
	// Circuit breaker key
	Key string `json:"key"`
	// Circuit breaker tenant id
	TenantId string `json:"tenant_id"`
	// Circuit breaker state
	State State `json:"state"`
	// Number of requests in the observability window
	Requests uint64 `json:"requests"`
	// Percentage of failures in the observability window
	FailureRate float64 `json:"failure_rate"`
	// Percentage of failures in the observability window
	SuccessRate float64 `json:"success_rate"`
	// Time after which the circuit breaker will reset when in half-open state
	WillResetAt time.Time `json:"will_reset_at"`
	// Number of failed requests in the observability window
	TotalFailures uint64 `json:"total_failures"`
	// Number of successful requests in the observability window
	TotalSuccesses uint64 `json:"total_successes"`
	// Number of consecutive circuit breaker trips
	ConsecutiveFailures uint64 `json:"consecutive_failures"`
	// Number of notifications (maximum of 3) sent in the observability window
	NotificationsSent uint64 `json:"notifications_sent"`

	logger *log.Logger
}

func NewCircuitBreaker(key string, tenantId string, logger *log.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		Key:               key,
		TenantId:          tenantId,
		State:             StateClosed,
		logger:            logger,
		NotificationsSent: 0,
	}
}

func NewCircuitBreakerFromStore(b []byte, logger *log.Logger) (*CircuitBreaker, error) {
	var c *CircuitBreaker
	innerErr := msgpack.DecodeMsgPack(b, &c)
	if innerErr != nil {
		return nil, innerErr
	}

	c.logger = logger

	return c, nil
}

func (b *CircuitBreaker) String() (s string) {
	bytes, err := msgpack.EncodeMsgPack(b)
	if err != nil {
		if b.logger != nil {
			b.logger.WithError(err).Error("[circuit breaker] failed to encode circuit breaker")
		}
		return ""
	}

	return string(bytes)
}

func (b *CircuitBreaker) asKeyValue() map[string]interface{} {
	kv := map[string]interface{}{}
	kv["key"] = b.Key
	kv["tenant_id"] = b.TenantId
	kv["state"] = b.State.String()
	kv["requests"] = b.Requests
	kv["failure_rate"] = b.FailureRate
	kv["success_rate"] = b.SuccessRate
	kv["will_reset_at"] = b.WillResetAt
	kv["total_failures"] = b.TotalFailures
	kv["total_successes"] = b.TotalSuccesses
	kv["consecutive_failures"] = b.ConsecutiveFailures
	kv["notifications_sent"] = b.NotificationsSent
	return kv
}

func (b *CircuitBreaker) trip(resetTime time.Time) {
	b.State = StateOpen
	b.WillResetAt = resetTime
	b.ConsecutiveFailures++
	if b.logger != nil {
		b.logger.Infof("[circuit breaker] circuit breaker transitioned to open.")
		b.logger.Debugf("[circuit breaker] circuit breaker state: %+v", b.asKeyValue())
	}
}

func (b *CircuitBreaker) toHalfOpen() {
	b.State = StateHalfOpen
	if b.logger != nil {
		b.logger.Infof("[circuit breaker] circuit breaker transitioned from open to half-open")
		b.logger.Debugf("[circuit breaker] circuit breaker state: %+v", b.asKeyValue())
	}
}

func (b *CircuitBreaker) Reset(resetTime time.Time) {
	b.State = StateClosed
	b.WillResetAt = resetTime
	b.NotificationsSent = 0
	b.ConsecutiveFailures = 0
	b.FailureRate = 0
	b.SuccessRate = 0
	b.TotalFailures = 0
	b.TotalSuccesses = 0
	b.Requests = 0
	if b.logger != nil {
		b.logger.Infof("[circuit breaker] circuit breaker transitioned from half-open to closed")
		b.logger.Debugf("[circuit breaker] circuit breaker state: %+v", b.asKeyValue())
	}
}
