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

const prefix = "breaker"

var (
	// ErrTooManyRequests is returned when the CB state is half open and the requests count is over the cb maxRequests
	ErrTooManyRequests = errors.New("too many requests")

	// ErrOpenState is returned when the CB state is open
	ErrOpenState = errors.New("circuit breaker is open")

	// ErrCircuitBreakerNotFound is returned when the circuit breaker is not found
	ErrCircuitBreakerNotFound = errors.New("circuit breaker not found")
)

// CircuitBreakerConfig is the configuration that all the circuit breakers will use
type CircuitBreakerConfig struct {
	// SampleRate is the time interval (in seconds) at which the data source
	// is polled to determine the number successful and failed requests
	SampleRate int `json:"sample_rate"`

	// ErrorTimeout is the time (in seconds) after which a circuit breaker goes
	// into the half-open state from the open state
	ErrorTimeout int `json:"error_timeout"`

	// FailureThreshold is the % of failed requests in the observability window
	// after which the breaker will go into the open state
	FailureThreshold float64 `json:"failure_threshold"`

	// FailureCount total number of failed requests in the observability window
	FailureCount int `json:"failure_count"`

	// SuccessThreshold is the % of successful requests in the observability window
	// after which a circuit breaker in the half-open state will go into the closed state
	SuccessThreshold int `json:"success_threshold"`

	// ObservabilityWindow is how far back in time (in minutes) the data source is
	// polled when determining the number successful and failed requests
	ObservabilityWindow int `json:"observability_window"`

	// NotificationThresholds These are the error counts after which we will send out notifications.
	NotificationThresholds []int `json:"notification_thresholds"`

	// ConsecutiveFailureThreshold determines when we ultimately disable the endpoint.
	// E.g., after 10 consecutive transitions from half-open → open we should disable it.
	ConsecutiveFailureThreshold int `json:"consecutive_failure_threshold"`
}

// State is a type that represents a state of CircuitBreaker.
type State int

func stateFromString(s string) State {
	switch s {
	case "open":
		return StateOpen
	case "closed":
		return StateClosed
	case "half-open":
		return StateHalfOpen
	}
	return StateClosed
}

// These constants are states of the CircuitBreaker.
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
	Requests             int       `json:"requests"`
	WillResetAt          time.Time `json:"will_reset_at"`
	TotalFailures        int       `json:"total_failures"`
	TotalSuccesses       int       `json:"total_successes"`
	ConsecutiveFailures  int       `json:"consecutive_failures"`
	ConsecutiveSuccesses int       `json:"consecutive_successes"`
	FailureRate          float64   `json:"failure_rate"`
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
	Failures  int    `json:"failures" db:"failures"`
	Successes int    `json:"successes" db:"successes"`
}

type CircuitBreakerManager struct {
	config   *CircuitBreakerConfig
	breakers []CircuitBreaker
	clock    clock.Clock
	redis    *redis.Client
}

func NewCircuitBreakerManager(client redis.UniversalClient) *CircuitBreakerManager {
	defaultConfig := &CircuitBreakerConfig{
		SampleRate:                  30,
		ErrorTimeout:                30,
		FailureThreshold:            0.1,
		FailureCount:                10,
		SuccessThreshold:            5,
		ObservabilityWindow:         5,
		NotificationThresholds:      []int{5, 10},
		ConsecutiveFailureThreshold: 10,
	}

	r := &CircuitBreakerManager{
		config: defaultConfig,
		clock:  clock.NewRealClock(),
		redis:  client.(*redis.Client),
	}

	return r
}

func (cb *CircuitBreakerManager) WithClock(c clock.Clock) *CircuitBreakerManager {
	cb.clock = c
	return cb
}

func (cb *CircuitBreakerManager) WithConfig(config *CircuitBreakerConfig) *CircuitBreakerManager {
	cb.config = config
	return cb
}

func (cb *CircuitBreakerManager) sampleStore(ctx context.Context, pollResults []PollResult) error {
	var keys []string
	for i := range pollResults {
		key := fmt.Sprintf("%s:%s", prefix, pollResults[i].Key)
		keys = append(keys, key)
		pollResults[i].Key = key
	}

	res, err := cb.redis.MGet(context.Background(), keys...).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}

	var circuitBreakers []CircuitBreaker
	for i := range res {
		if res[i] == nil {
			c := CircuitBreaker{
				State: StateClosed,
				Key:   pollResults[i].Key,
			}
			circuitBreakers = append(circuitBreakers, c)
			continue
		}

		c := CircuitBreaker{}
		asBytes := []byte(res[i].(string))
		innerErr := msgpack.DecodeMsgPack(asBytes, &c)
		if innerErr != nil {
			return innerErr
		}

		circuitBreakers = append(circuitBreakers, c)
	}

	resultsMap := make(map[string]PollResult)
	for _, result := range pollResults {
		resultsMap[result.Key] = result
	}

	circuitBreakerMap := make(map[string]CircuitBreaker, len(resultsMap))

	for _, breaker := range circuitBreakers {
		result := resultsMap[breaker.Key]

		breaker.Requests = result.Successes + result.Failures
		breaker.TotalFailures = result.Failures
		breaker.TotalSuccesses = result.Successes
		breaker.FailureRate = float64(breaker.TotalFailures) / float64(breaker.TotalSuccesses+breaker.TotalFailures)

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
		log.WithError(err).Error("failed to update state")
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
	return cb.redis.MSet(ctx, breakerStringsMap).Err()
}

func (cb *CircuitBreakerManager) loadCircuitBreakers(ctx context.Context) ([]CircuitBreaker, error) {
	keys, err := cb.redis.Keys(ctx, "breaker*").Result()
	if err != nil {
		return nil, err
	}

	res, err := cb.redis.MGet(ctx, keys...).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
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

// GetCircuitBreaker is used to get fetch the circuit breaker state before executing a function
func (cb *CircuitBreakerManager) GetCircuitBreaker(ctx context.Context, key string) (c *CircuitBreaker, err error) {
	breakerKey := fmt.Sprintf("%s:%s", prefix, key)

	res, err := cb.redis.Get(ctx, breakerKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	if err != nil && errors.Is(err, redis.Nil) {
		return nil, ErrCircuitBreakerNotFound
	}

	err = msgpack.DecodeMsgPack([]byte(res), &c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (cb *CircuitBreakerManager) Start(ctx context.Context, poolFunc func(ctx context.Context, lookBackDuration int) (results []PollResult, err error)) {
	for {
		// Get the failure and success counts from the last X minutes
		pollResults, err := poolFunc(ctx, cb.config.ObservabilityWindow)
		if err != nil {
			log.WithError(err).Error("poll db failed")
			time.Sleep(time.Duration(cb.config.SampleRate) * time.Second)
			continue
		}

		if len(pollResults) == 0 {
			// there's nothing to update
			time.Sleep(time.Duration(cb.config.SampleRate) * time.Second)
			continue
		}

		err = cb.sampleStore(ctx, pollResults)
		if err != nil {
			log.WithError(err).Error("Failed to sample events and update state")
		}
		time.Sleep(time.Duration(cb.config.SampleRate) * time.Second)
	}
}
