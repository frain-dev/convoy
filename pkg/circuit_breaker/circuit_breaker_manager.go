package circuit_breaker

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"strings"
	"time"
)

const prefix = "breaker:"
const mutexKey = "convoy:circuit_breaker:mutex"

type PollFunc func(ctx context.Context, lookBackDuration uint64, resetTimes map[string]time.Time) (map[string]PollResult, error)
type CircuitBreakerOption func(cb *CircuitBreakerManager) error

var (
	// ErrTooManyRequests is returned when the circuit breaker state is half open and the request count is over the failureThreshold
	ErrTooManyRequests = errors.New("[circuit breaker] too many requests")

	// ErrOpenState is returned when the circuit breaker state is open
	ErrOpenState = errors.New("[circuit breaker] circuit breaker is open")

	// ErrCircuitBreakerNotFound is returned when the circuit breaker is not found
	ErrCircuitBreakerNotFound = errors.New("[circuit breaker] circuit breaker not found")

	// ErrClockMustNotBeNil is returned when a nil clock is passed to NewCircuitBreakerManager
	ErrClockMustNotBeNil = errors.New("[circuit breaker] clock must not be nil")

	// ErrStoreMustNotBeNil is returned when a nil store is passed to NewCircuitBreakerManager
	ErrStoreMustNotBeNil = errors.New("[circuit breaker] store must not be nil")

	// ErrConfigMustNotBeNil is returned when a nil config is passed to NewCircuitBreakerManager
	ErrConfigMustNotBeNil = errors.New("[circuit breaker] config must not be nil")

	// ErrNotificationFunctionMustNotBeNil is returned when a nil function is passed to NewCircuitBreakerManager
	ErrNotificationFunctionMustNotBeNil = errors.New("[circuit breaker] notification function must not be nil")
)

// State represents a state of a CircuitBreaker.
type State int
type NotificationType string

// These are the states of a CircuitBreaker.
const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

const (
	TypeDisableResource  NotificationType = "disable"
	TypeTriggerThreshold NotificationType = "trigger"
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

type PollResult struct {
	Key       string `json:"key" db:"key"`
	TenantId  string `json:"tenant_id" db:"tenant_id"`
	Failures  uint64 `json:"failures" db:"failures"`
	Successes uint64 `json:"successes" db:"successes"`
}

type CircuitBreakerManager struct {
	config         *CircuitBreakerConfig
	clock          clock.Clock
	store          CircuitBreakerStore
	notificationFn func(NotificationType, CircuitBreakerConfig, CircuitBreaker) error
}

func NewCircuitBreakerManager(options ...CircuitBreakerOption) (*CircuitBreakerManager, error) {
	r := &CircuitBreakerManager{}

	for _, opt := range options {
		err := opt(r)
		if err != nil {
			return r, err
		}
	}

	if r.store == nil {
		return nil, ErrStoreMustNotBeNil
	}

	if r.clock == nil {
		return nil, ErrClockMustNotBeNil
	}

	if r.config == nil {
		return nil, ErrConfigMustNotBeNil
	}

	return r, nil
}

func StoreOption(store CircuitBreakerStore) CircuitBreakerOption {
	return func(cb *CircuitBreakerManager) error {
		if store == nil {
			return ErrStoreMustNotBeNil
		}

		cb.store = store
		return nil
	}
}

func ClockOption(clock clock.Clock) CircuitBreakerOption {
	return func(cb *CircuitBreakerManager) error {
		if clock == nil {
			return ErrClockMustNotBeNil
		}

		cb.clock = clock
		return nil
	}
}

func ConfigOption(config *CircuitBreakerConfig) CircuitBreakerOption {
	return func(cb *CircuitBreakerManager) error {
		if config == nil {
			return ErrConfigMustNotBeNil
		}

		if err := config.Validate(); err != nil {
			return err
		}

		cb.config = config
		return nil
	}
}

func NotificationFunctionOption(fn func(NotificationType, CircuitBreakerConfig, CircuitBreaker) error) CircuitBreakerOption {
	return func(cb *CircuitBreakerManager) error {
		if fn == nil {
			return ErrNotificationFunctionMustNotBeNil
		}

		cb.notificationFn = fn
		return nil
	}
}

func (cb *CircuitBreakerManager) sampleStore(ctx context.Context, pollResults map[string]PollResult) error {
	redisCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	circuitBreakers := make(map[string]CircuitBreaker, len(pollResults))

	keys, tenants, j := make([]string, len(pollResults)), make([]string, len(pollResults)), 0
	for k := range pollResults {
		tenants[j] = pollResults[k].TenantId

		key := fmt.Sprintf("%s%s", prefix, k)
		keys[j] = key
		j++
	}

	res, err := cb.store.GetMany(redisCtx, keys...)
	if err != nil {
		return err
	}

	for i := range res {
		if res[i] == nil {
			circuitBreakers[keys[i]] = *NewCircuitBreaker(keys[i], tenants[i])
			continue
		}

		str, ok := res[i].(string)
		if !ok {
			log.Errorf("[circuit breaker] breaker with key (%s) is corrupted, reseting it", keys[i])

			// the circuit breaker is corrupted, create a new one in its place
			circuitBreakers[keys[i]] = *NewCircuitBreaker(keys[i], tenants[i])
			continue
		}

		var c CircuitBreaker
		asBytes := []byte(str)
		innerErr := msgpack.DecodeMsgPack(asBytes, &c)
		if innerErr != nil {
			return innerErr
		}

		circuitBreakers[keys[i]] = c
	}

	for key, breaker := range circuitBreakers {
		k := strings.Split(key, ":")
		result := pollResults[k[1]]

		breaker.TotalFailures = result.Failures
		breaker.TotalSuccesses = result.Successes
		breaker.Requests = breaker.TotalSuccesses + breaker.TotalFailures

		prevFailureRate := breaker.FailureRate
		if breaker.Requests == 0 {
			breaker.FailureRate = 0
			breaker.SuccessRate = 0
		} else {
			breaker.FailureRate = float64(breaker.TotalFailures) / float64(breaker.Requests) * 100
			breaker.SuccessRate = float64(breaker.TotalSuccesses) / float64(breaker.Requests) * 100
		}

		if breaker.State == StateHalfOpen && breaker.SuccessRate >= float64(cb.config.SuccessThreshold) {
			breaker.resetCircuitBreaker()
		} else if (breaker.State == StateClosed || breaker.State == StateHalfOpen) && breaker.Requests >= cb.config.MinimumRequestCount {
			if breaker.FailureRate >= float64(cb.config.FailureThreshold) {
				breaker.tripCircuitBreaker(cb.clock.Now().Add(time.Duration(cb.config.BreakerTimeout) * time.Second))
			}
		}

		if breaker.State == StateOpen && cb.clock.Now().After(breaker.WillResetAt) {
			breaker.toHalfOpen()
		}

		// send notifications for each circuit breaker
		if cb.notificationFn != nil {
			if prevFailureRate < breaker.FailureRate && breaker.NotificationsSent < 3 {
				if breaker.FailureRate >= float64(cb.config.NotificationThresholds[breaker.NotificationsSent]) {
					innerErr := cb.notificationFn(TypeTriggerThreshold, *cb.config, breaker)
					if innerErr != nil {
						log.WithError(innerErr).Errorf("[circuit breaker] failed to execute threshold notification function")
					}
					log.Debugf("[circuit breaker] executed threshold notification function at %v", cb.config.NotificationThresholds[breaker.NotificationsSent])
					breaker.NotificationsSent++
				}
			}

			if breaker.ConsecutiveFailures >= cb.GetConfig().ConsecutiveFailureThreshold {
				innerErr := cb.notificationFn(TypeDisableResource, *cb.config, breaker)
				if innerErr != nil {
					log.WithError(innerErr).Errorf("[circuit breaker] failed to execute disable resource notification function")
				}
				log.Debug("[circuit breaker] executed disable resource notification function")
			}
		}

		circuitBreakers[key] = breaker
	}

	if err = cb.updateCircuitBreakers(ctx, circuitBreakers); err != nil {
		log.WithError(err).Error("[circuit breaker] failed to update state")
		return err
	}

	return nil
}

func (cb *CircuitBreakerManager) updateCircuitBreakers(ctx context.Context, breakers map[string]CircuitBreaker) (err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return cb.store.SetMany(ctx, breakers, time.Duration(cb.config.ObservabilityWindow)*time.Minute)
}

func (cb *CircuitBreakerManager) loadCircuitBreakers(ctx context.Context) ([]CircuitBreaker, error) {
	redisCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	keys, err := cb.store.Keys(redisCtx, prefix)
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return []CircuitBreaker{}, nil
	}

	redisCtx2, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()

	res, err := cb.store.GetMany(redisCtx2, keys...)
	if err != nil {
		return nil, err
	}

	circuitBreakers := make([]CircuitBreaker, len(res))
	for i := range res {
		c := CircuitBreaker{}
		switch res[i].(type) {
		case string:
			asBytes := []byte(res[i].(string))
			innerErr := msgpack.DecodeMsgPack(asBytes, &c)
			if innerErr != nil {
				return nil, innerErr
			}
		case CircuitBreaker:
			c = res[i].(CircuitBreaker)
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
		if b.FailureRate > float64(cb.config.FailureThreshold) {
			return ErrTooManyRequests
		}
		return nil
	default:
		return nil
	}
}

// CanExecute checks if the circuit breaker for a key will return an error for the current state.
// It will not return an error if it is in the closed state or half-open state when the failure
// threshold has not been reached, it will also fail-open if the circuit breaker is not found.
func (cb *CircuitBreakerManager) CanExecute(ctx context.Context, key string) error {
	b, err := cb.GetCircuitBreaker(ctx, key)
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

// GetCircuitBreaker is used to get fetch the circuit breaker state,
// it fails open if the circuit breaker for that key is not found
func (cb *CircuitBreakerManager) GetCircuitBreaker(ctx context.Context, key string) (c *CircuitBreaker, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	bKey := fmt.Sprintf("%s%s", prefix, key)
	res, err := cb.store.GetOne(ctx, bKey)
	if err != nil {
		if errors.Is(err, ErrCircuitBreakerNotFound) {
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

// GetCircuitBreakerWithError is used to get fetch the circuit breaker state,
// it returns ErrCircuitBreakerNotFound when a circuit breaker for the key is not found
func (cb *CircuitBreakerManager) GetCircuitBreakerWithError(ctx context.Context, key string) (c *CircuitBreaker, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	bKey := fmt.Sprintf("%s%s", prefix, key)
	res, err := cb.store.GetOne(ctx, bKey)
	if err != nil {
		return nil, err
	}

	err = msgpack.DecodeMsgPack([]byte(res), &c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (cb *CircuitBreakerManager) sampleAndUpdate(ctx context.Context, pollFunc PollFunc) error {
	mu, err := cb.store.Lock(ctx, mutexKey)
	if err != nil {
		log.WithError(err).Error("[circuit breaker] failed to acquire lock")
		return err
	}

	defer func() {
		innerErr := cb.store.Unlock(ctx, mu)
		if innerErr != nil {
			log.WithError(innerErr).Error("[circuit breaker] failed to unlock mutex")
		}
	}()

	bs, err := cb.loadCircuitBreakers(ctx)
	if err != nil {
		log.WithError(err).Error("[circuit breaker] failed to load circuitBreakers")
		return err
	}

	resetMap := make(map[string]time.Time, len(bs))
	for i := range bs {
		if bs[i].State == StateClosed && bs[i].WillResetAt.After(time.Time{}) {
			resetMap[bs[i].Key] = bs[i].WillResetAt
		}
	}

	// Get the failure and success counts from the last X minutes
	pollResults, err := pollFunc(ctx, cb.config.ObservabilityWindow, resetMap)
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

func (cb *CircuitBreakerManager) GetConfig() CircuitBreakerConfig {
	return *cb.config
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
