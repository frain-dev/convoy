package circuit_breaker

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
)

const prefix = "breaker:"

type PollFunc func(ctx context.Context, lookBackDuration uint64) ([]PollResult, error)

var (
	// ErrTooManyRequests is returned when the circuit breaker state is half open and the request count is over the failureThreshold
	ErrTooManyRequests = errors.New("[circuit breaker] too many requests")

	// ErrOpenState is returned when the circuit breaker state is open
	ErrOpenState = errors.New("[circuit breaker] circuit breaker is open")

	// ErrCircuitBreakerNotFound is returned when the circuit breaker is not found
	ErrCircuitBreakerNotFound = errors.New("[circuit breaker] circuit breaker not found")

	// ErrClockMustNotBeNil is returned when a nil clock is passed to NewCircuitBreakerManager
	ErrClockMustNotBeNil = errors.New("[circuit breaker] clock must not be nil")

	// ErrConfigMustNotBeNil is returned when a nil config is passed to NewCircuitBreakerManager
	ErrConfigMustNotBeNil = errors.New("[circuit breaker] config must not be nil")
)

// CircuitBreakerConfig is the configuration that all the circuit breakers will use
type CircuitBreakerConfig struct {
	// SampleRate is the time interval (in seconds) at which the data source
	// is polled to determine the number successful and failed requests
	SampleRate uint64 `json:"sample_rate"`

	// ErrorTimeout is the time (in seconds) after which a circuit breaker goes
	// into the half-open state from the open state
	ErrorTimeout uint64 `json:"error_timeout"`

	// FailureThreshold is the % of failed requests in the observability window
	// after which the breaker will go into the open state
	FailureThreshold float64 `json:"failure_threshold"`

	// FailureCount total number of failed requests in the observability window
	FailureCount uint64 `json:"failure_count"`

	// SuccessThreshold is the % of successful requests in the observability window
	// after which a circuit breaker in the half-open state will go into the closed state
	SuccessThreshold uint64 `json:"success_threshold"`

	// ObservabilityWindow is how far back in time (in minutes) the data source is
	// polled when determining the number successful and failed requests
	ObservabilityWindow uint64 `json:"observability_window"`

	// NotificationThresholds These are the error counts after which we will send out notifications.
	NotificationThresholds []uint64 `json:"notification_thresholds"`

	// ConsecutiveFailureThreshold determines when we ultimately disable the endpoint.
	// E.g., after 10 consecutive transitions from half-open → open we should disable it.
	ConsecutiveFailureThreshold uint64 `json:"consecutive_failure_threshold"`
}

func (c *CircuitBreakerConfig) Validate() error {
	var errs strings.Builder

	if c.SampleRate == 0 {
		errs.WriteString("SampleRate must be greater than 0")
		errs.WriteString("; ")
	}

	if c.ErrorTimeout == 0 {
		errs.WriteString("ErrorTimeout must be greater than 0")
		errs.WriteString("; ")
	}

	if c.FailureThreshold < 0 || c.FailureThreshold > 1 {
		errs.WriteString("FailureThreshold must be between 0 and 1")
		errs.WriteString("; ")
	}

	if c.FailureCount == 0 {
		errs.WriteString("FailureCount must be greater than 0")
		errs.WriteString("; ")
	}

	if c.SuccessThreshold == 0 {
		errs.WriteString("SuccessThreshold must be greater than 0")
		errs.WriteString("; ")
	}

	if c.ObservabilityWindow == 0 {
		errs.WriteString("ObservabilityWindow must be greater than 0")
		errs.WriteString("; ")
	}

	if c.ObservabilityWindow <= c.SampleRate {
		errs.WriteString("ObservabilityWindow must be greater than the SampleRate")
		errs.WriteString("; ")
	}

	if len(c.NotificationThresholds) == 0 {
		errs.WriteString("NotificationThresholds must contain at least one threshold")
		errs.WriteString("; ")
	} else {
		for i := 0; i < len(c.NotificationThresholds); i++ {
			if c.NotificationThresholds[i] == 0 {
				errs.WriteString(fmt.Sprintf("Notification thresholds at index [%d] = %d must be greater than 0", i, c.NotificationThresholds[i]))
				errs.WriteString("; ")
			}
		}

		for i := 0; i < len(c.NotificationThresholds)-1; i++ {
			if c.NotificationThresholds[i] >= c.NotificationThresholds[i+1] {
				errs.WriteString("NotificationThresholds should be in ascending order")
				errs.WriteString("; ")
			}
		}
	}

	if c.ConsecutiveFailureThreshold == 0 {
		errs.WriteString("ConsecutiveFailureThreshold must be greater than 0")
		errs.WriteString("; ")
	}

	if errs.Len() > 0 {
		return fmt.Errorf("config validation failed with errors: %s", errs.String())
	}

	return nil
}

// State represents a state of a CircuitBreaker.
type State int

// These are the states of a CircuitBreaker.
const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return fmt.Sprintf("unknown state: %d", s)
	}
}

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

type PollResult struct {
	Key       string `json:"key" db:"key"`
	Failures  uint64 `json:"failures" db:"failures"`
	Successes uint64 `json:"successes" db:"successes"`
}

type CircuitBreakerManager struct {
	config *CircuitBreakerConfig
	clock  clock.Clock
	redis  RedisClient
}

type RedisClient interface {
	Keys(ctx context.Context, pattern string) *redis.StringSliceCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	MGet(ctx context.Context, keys ...string) *redis.SliceCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
	TxPipeline() redis.Pipeliner
}

func NewCircuitBreakerManager(client RedisClient) *CircuitBreakerManager {
	defaultConfig := &CircuitBreakerConfig{
		SampleRate:                  30,
		ErrorTimeout:                30,
		FailureThreshold:            0.1,
		FailureCount:                10,
		SuccessThreshold:            5,
		ObservabilityWindow:         5,
		NotificationThresholds:      []uint64{5, 10},
		ConsecutiveFailureThreshold: 10,
	}

	r := &CircuitBreakerManager{
		config: defaultConfig,
		clock:  clock.NewRealClock(),
		redis:  client,
	}

	return r
}

func (cb *CircuitBreakerManager) WithClock(c clock.Clock) (*CircuitBreakerManager, error) {
	if cb.clock == nil {
		return nil, ErrClockMustNotBeNil
	}

	cb.clock = c
	return cb, nil
}

func (cb *CircuitBreakerManager) WithConfig(config *CircuitBreakerConfig) (*CircuitBreakerManager, error) {
	if config == nil {
		return nil, ErrConfigMustNotBeNil
	}

	cb.config = config

	if err := config.Validate(); err != nil {
		return nil, err
	}
	return cb, nil
}

func (cb *CircuitBreakerManager) sampleStore(ctx context.Context, pollResults []PollResult) error {
	var keys []string
	for i := range pollResults {
		key := fmt.Sprintf("%s%s", prefix, pollResults[i].Key)
		keys = append(keys, key)
		pollResults[i].Key = key
	}

	deadlineCtx, cancel := context.WithDeadline(ctx, cb.clock.Now().Add(5*time.Second))
	defer cancel()

	res, err := cb.redis.MGet(deadlineCtx, keys...).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return err
	}

	circuitBreakers := make([]CircuitBreaker, len(pollResults))
	for i := range res {
		if res[i] == nil {
			c := CircuitBreaker{
				State: StateClosed,
				Key:   pollResults[i].Key,
			}
			circuitBreakers[i] = c
			continue
		}

		c := CircuitBreaker{}
		str, ok := res[i].(string)
		if !ok {
			log.Errorf("[circuit breaker] breaker with key (%s) is corrupted, reseting it", keys[i])

			// the circuit breaker is corrupted, create a new one in its place
			circuitBreakers[i] = CircuitBreaker{
				State: StateClosed,
				Key:   keys[i],
			}
			continue
		}

		asBytes := []byte(str)
		innerErr := msgpack.DecodeMsgPack(asBytes, &c)
		if innerErr != nil {
			return innerErr
		}

		circuitBreakers[i] = c
	}

	resultsMap := make(map[string]PollResult)
	for _, result := range pollResults {
		resultsMap[result.Key] = result
	}

	circuitBreakerMap := make(map[string]CircuitBreaker, len(resultsMap))

	for _, breaker := range circuitBreakers {
		result := resultsMap[breaker.Key]

		breaker.TotalFailures = result.Failures
		breaker.TotalSuccesses = result.Successes
		breaker.Requests = breaker.TotalSuccesses + breaker.TotalFailures

		if breaker.Requests == 0 {
			breaker.FailureRate = 0
		} else {
			breaker.FailureRate = float64(breaker.TotalFailures) / float64(breaker.Requests)
		}

		if breaker.State == StateHalfOpen && breaker.TotalSuccesses >= cb.config.SuccessThreshold {
			breaker.resetCircuitBreaker()
		} else if breaker.State == StateClosed && (breaker.FailureRate >= cb.config.FailureThreshold || breaker.TotalFailures >= cb.config.FailureCount) {
			breaker.tripCircuitBreaker(cb.clock.Now().Add(time.Duration(cb.config.ErrorTimeout) * time.Second))
		}

		if breaker.State == StateOpen && cb.clock.Now().After(breaker.WillResetAt) {
			breaker.toHalfOpen()
		}

		circuitBreakerMap[breaker.Key] = breaker
	}

	if err = cb.updateCircuitBreakers(ctx, circuitBreakerMap); err != nil {
		log.WithError(err).Error("[circuit breaker] failed to update state")
		return err
	}

	return nil
}

func (cb *CircuitBreakerManager) updateCircuitBreakers(ctx context.Context, breakers map[string]CircuitBreaker) (err error) {
	deadlineCtx, cancel := context.WithDeadline(ctx, cb.clock.Now().Add(5*time.Second))
	defer cancel()

	pipe := cb.redis.TxPipeline()
	for key, breaker := range breakers {
		val, innerErr := breaker.String()
		if innerErr != nil {
			return innerErr
		}

		if innerErr = pipe.Set(deadlineCtx, key, val, time.Duration(cb.config.ObservabilityWindow)*time.Minute).Err(); innerErr != nil {
			return innerErr
		}
	}

	_, err = pipe.Exec(deadlineCtx)
	if err != nil {
		return err
	}

	return nil
}

func (cb *CircuitBreakerManager) loadCircuitBreakers(ctx context.Context) ([]CircuitBreaker, error) {
	deadlineCtx, cancel := context.WithDeadline(ctx, cb.clock.Now().Add(5*time.Second))
	defer cancel()

	keys, err := cb.redis.Keys(deadlineCtx, "breaker*").Result()
	if err != nil {
		return nil, err
	}

	res, err := cb.redis.MGet(deadlineCtx, keys...).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	circuitBreakers := make([]CircuitBreaker, len(res))
	for i := range res {
		c := CircuitBreaker{}
		asBytes := []byte(res[i].(string))
		innerErr := msgpack.DecodeMsgPack(asBytes, &c)
		if innerErr != nil {
			return nil, innerErr
		}

		circuitBreakers[i] = c
	}

	return circuitBreakers, nil
}

func (cb *CircuitBreakerManager) getCircuitBreakerError(b CircuitBreaker) error {
	switch b.State {
	case StateOpen:
		return ErrOpenState
	case StateHalfOpen:
		if b.TotalFailures > cb.config.FailureCount {
			return ErrTooManyRequests
		}
		return nil
	default:
		return nil
	}
}

// CanExecute checks if the circuit breaker for a key will return an error for the current state.
// It will not return an error if it is in the closed state or half-open state when the failure
// threshold has not been reached, it will fail-open if the circuit breaker is not found
func (cb *CircuitBreakerManager) CanExecute(ctx context.Context, key string) error {
	b, err := cb.getCircuitBreaker(ctx, key)
	if err != nil {
		return err
	}

	if b != nil {
		switch b.State {
		case StateOpen, StateHalfOpen:
			return cb.getCircuitBreakerError(*b)
		default:
			return nil
		}
	}

	return nil
}

// getCircuitBreaker is used to get fetch the circuit breaker state,
// it fails open if the circuit breaker for that key is not found
func (cb *CircuitBreakerManager) getCircuitBreaker(ctx context.Context, key string) (c *CircuitBreaker, err error) {
	deadlineCtx, cancel := context.WithDeadline(ctx, cb.clock.Now().Add(5*time.Second))
	defer cancel()

	bKey := fmt.Sprintf("%s%s", prefix, key)
	res, err := cb.redis.Get(deadlineCtx, bKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// a circuit breaker was not found for this key;
			// it probably hasn't been created;
			// we should fail open
			return nil, nil
		}
		return nil, err
	}

	err = msgpack.DecodeMsgPack([]byte(res), &c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// GetCircuitBreaker is used to get fetch the circuit breaker state,
// it returns ErrCircuitBreakerNotFound when a circuit breaker for the key is not found
func (cb *CircuitBreakerManager) GetCircuitBreaker(ctx context.Context, key string) (c *CircuitBreaker, err error) {
	deadlineCtx, cancel := context.WithDeadline(ctx, cb.clock.Now().Add(5*time.Second))
	defer cancel()

	bKey := fmt.Sprintf("%s%s", prefix, key)
	res, err := cb.redis.Get(deadlineCtx, bKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCircuitBreakerNotFound
		}
		return nil, err
	}

	err = msgpack.DecodeMsgPack([]byte(res), &c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (cb *CircuitBreakerManager) sampleAndUpdate(ctx context.Context, pollFunc PollFunc) error {
	// Get the failure and success counts from the last X minutes
	pollResults, err := pollFunc(ctx, cb.config.ObservabilityWindow)
	if err != nil {
		return fmt.Errorf("poll function failed: %w", err)
	}

	if len(pollResults) == 0 {
		return nil // Nothing to update
	}

	if err = cb.sampleStore(ctx, pollResults); err != nil {
		return fmt.Errorf("[circuit breaker] failed to sample events and update state: %w", err)
	}

	return nil
}

func (cb *CircuitBreakerManager) Start(ctx context.Context, pollFunc PollFunc) {
	ticker := time.NewTicker(time.Duration(cb.config.SampleRate) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := cb.sampleAndUpdate(ctx, pollFunc); err != nil {
				log.WithError(err).Error("[circuit breaker] failed to sample and update circuit breakers")
			}
		}
	}
}
