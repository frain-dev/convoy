package circuit_breaker

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/redis/go-redis/v9"
	"time"
)

const (
	prefix = "breaker:"
	keyTTL = time.Hour
)

type PollFunc func(ctx context.Context, lookBackDuration uint64) ([]PollResult, error)

var (
	// ErrTooManyRequests is returned when the CB state is half open and the request count is over the failureThreshold
	ErrTooManyRequests = errors.New("[circuit breaker] too many requests")

	// ErrOpenState is returned when the CB state is open
	ErrOpenState = errors.New("[circuit breaker] circuit breaker is open")

	// ErrCircuitBreakerNotFound is returned when the CB is not found
	ErrCircuitBreakerNotFound = errors.New("[circuit breaker] circuit breaker not found")
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
	var errs []string

	if c.SampleRate == 0 {
		errs = append(errs, "SampleRate must be greater than 0")
	}

	if c.ErrorTimeout == 0 {
		errs = append(errs, "ErrorTimeout must be greater than 0")
	}

	if c.FailureThreshold < 0 || c.FailureThreshold > 1 {
		errs = append(errs, "FailureThreshold must be between 0 and 1")
	}

	if c.FailureCount == 0 {
		errs = append(errs, "FailureCount must be greater than 0")
	}

	if c.SuccessThreshold == 0 {
		errs = append(errs, "SuccessThreshold must be greater than 0")
	}

	if c.ObservabilityWindow == 0 {
		errs = append(errs, "ObservabilityWindow must be greater than 0")
	}

	if len(c.NotificationThresholds) == 0 {
		errs = append(errs, "NotificationThresholds must contain at least one threshold")
	} else {
		for i, threshold := range c.NotificationThresholds {
			if threshold == 0 {
				errs = append(errs, fmt.Sprintf("Notification thresholds at index [%d] = %d must be greater than 0", i, threshold))
			}
		}
	}

	if c.ConsecutiveFailureThreshold == 0 {
		errs = append(errs, "ConsecutiveFailureThreshold must be greater than 0")
	}

	if len(errs) > 0 {
		return errors.New("CircuitBreakerConfig validation failed: " + joinErrors(errs))
	}

	return nil
}

func joinErrors(errs []string) string {
	result := ""
	for i, err := range errs {
		if i > 0 {
			result += "; "
		}
		result += err
	}
	return result
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
	Key                  string    `json:"key"`
	State                State     `json:"state"`
	Requests             uint64    `json:"requests"`
	FailureRate          float64   `json:"failure_rate"`
	WillResetAt          time.Time `json:"will_reset_at"`
	TotalFailures        uint64    `json:"total_failures"`
	TotalSuccesses       uint64    `json:"total_successes"`
	ConsecutiveFailures  uint64    `json:"consecutive_failures"`
	ConsecutiveSuccesses uint64    `json:"consecutive_successes"`
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
	b.ConsecutiveSuccesses++
}

type PollResult struct {
	Key       string `json:"key" db:"key"`
	Failures  uint64 `json:"failures" db:"failures"`
	Successes uint64 `json:"successes" db:"successes"`
}

type CircuitBreakerManager struct {
	config *CircuitBreakerConfig
	clock  clock.Clock
	redis  *redis.Client
}

func NewCircuitBreakerManager(client redis.UniversalClient) *CircuitBreakerManager {
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
		redis:  client.(*redis.Client),
	}

	return r
}

func (cb *CircuitBreakerManager) WithClock(c clock.Clock) (*CircuitBreakerManager, error) {
	if cb.clock == nil {
		return nil, errors.New("clock must not be nil")
	}

	cb.clock = c
	return cb, nil
}

func (cb *CircuitBreakerManager) WithConfig(config *CircuitBreakerConfig) (*CircuitBreakerManager, error) {
	if config == nil {
		return nil, errors.New("config must not be nil")
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
		key := fmt.Sprintf("%s:%s", prefix, pollResults[i].Key)
		keys = append(keys, key)
		pollResults[i].Key = key
	}

	res, err := cb.redis.MGet(context.Background(), keys...).Result()
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
			log.Errorf("[circuit breaker] breaker with key (%s) is corrupted", keys[i])

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

func (cb *CircuitBreakerManager) updateCircuitBreakers(ctx context.Context, breakers map[string]CircuitBreaker) error {
	breakerStringsMap := make(map[string]string, len(breakers))
	for key, breaker := range breakers {
		val, err := breaker.String()
		if err != nil {
			return err
		}
		breakerStringsMap[key] = val
	}

	// Update the state
	err := cb.redis.MSet(ctx, breakerStringsMap).Err()
	if err != nil {
		return err
	}

	pipe := cb.redis.TxPipeline()
	for key := range breakers {
		err = pipe.Expire(ctx, key, keyTTL).Err()
		if err != nil {
			return err
		}
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (cb *CircuitBreakerManager) loadCircuitBreakers(ctx context.Context) ([]CircuitBreaker, error) {
	keys, err := cb.redis.Keys(ctx, "breaker*").Result()
	if err != nil {
		return nil, err
	}

	res, err := cb.redis.MGet(ctx, keys...).Result()
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

func (cb *CircuitBreakerManager) GetCircuitBreakerError(ctx context.Context, key string) error {
	b, err := cb.GetCircuitBreaker(ctx, key)
	if err != nil {
		return err
	}

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

// GetCircuitBreaker is used to get fetch the circuit breaker state before executing a function
func (cb *CircuitBreakerManager) GetCircuitBreaker(ctx context.Context, key string) (c *CircuitBreaker, err error) {
	breakerKey := fmt.Sprintf("%s%s", prefix, key)

	res, err := cb.redis.Get(ctx, breakerKey).Result()
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

func (cb *CircuitBreakerManager) cleanup(ctx context.Context) error {
	keys, err := cb.redis.Keys(ctx, fmt.Sprintf("%s%s", prefix, "*")).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys for cleanup: %w", err)
	}

	pipe := cb.redis.TxPipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute cleanup pipeline: %w", err)
	}

	return nil
}

func (cb *CircuitBreakerManager) Start(ctx context.Context, pollFunc PollFunc) error {
	ticker := time.NewTicker(time.Duration(cb.config.SampleRate) * time.Second)
	defer ticker.Stop()

	// Run cleanup daily
	// todo(raymond): should this be run by asynq?
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := cb.sampleAndUpdate(ctx, pollFunc); err != nil {
				log.WithError(err).Error("[circuit breaker] failed to sample and update circuit breakers")
			}
		case <-cleanupTicker.C:
			if err := cb.cleanup(ctx); err != nil {
				log.WithError(err).Error("[circuit breaker] failed to cleanup circuit breakers")
			}
		}
	}
}
