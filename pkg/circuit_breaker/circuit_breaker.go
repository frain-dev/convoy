package circuit_breaker

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"time"
)

const prefix = "breaker"

var (
	// ErrTooManyRequests is returned when the CB state is half open and the requests count is over the cb maxRequests
	ErrTooManyRequests = errors.New("too many requests")
	// ErrOpenState is returned when the CB state is open
	ErrOpenState = errors.New("circuit breaker is open")
)

// CircuitBreakerConfig is config which all the circuit breakers that manager manages will use
//
//	{
//		"sample_time": 5,
//		"duration": 5,
//		"error_timeout": 50,
//		"error_threshold": 70,
//		"failure_count": 10,
//		"success_threshold": 10,
//		"consecutive_failure_threshold": 10,
//		"notification_thresholds": [30, 65]
//	}
type CircuitBreakerConfig struct {
	// SampleTime is the time interval (in seconds) at which the data source
	// is polled to determine the number successful and failed requests
	SampleTime int `json:"sample_time"`

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

	// ObservabilityWindow is how far back in time (in seconds) the data source is
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
// todo(raymond): implement methods to check the state and find out if an action can be performed.
type CircuitBreaker struct {
	State                State     `json:"state"`
	Requests             int       `json:"requests"`
	EndpointID           string    `json:"endpoint_id"`
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

type DBPollResult struct {
	EndpointID string `json:"endpoint_id" db:"endpoint_id"`
	Failures   int    `json:"failures" db:"failures"`
	Successes  int    `json:"successes" db:"successes"`
}

type CircuitBreakerManager struct {
	breakers []CircuitBreaker
	config   CircuitBreakerConfig
	clock    clock.Clock
	redis    *redis.Client
	db       *sqlx.DB
}

func NewCircuitBreakerManager(client redis.UniversalClient, db *sqlx.DB, clock clock.Clock, config CircuitBreakerConfig) *CircuitBreakerManager {
	r := &CircuitBreakerManager{
		db:     db,
		clock:  clock,
		config: config,
		redis:  client.(*redis.Client),
	}

	return r
}

func (cb *CircuitBreakerManager) sampleEventsAndUpdateState(ctx context.Context, dbPollResults []DBPollResult) error {
	var keys []string
	for i := range dbPollResults {
		key := fmt.Sprintf("%s:%s", prefix, dbPollResults[i].EndpointID)
		keys = append(keys, key)
		dbPollResults[i].EndpointID = key
	}

	res, err := cb.redis.MGet(context.Background(), keys...).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}

	var circuitBreakers []CircuitBreaker
	for i := range res {
		if res[i] == nil {
			c := CircuitBreaker{
				State:      StateClosed,
				EndpointID: dbPollResults[i].EndpointID,
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

	resultsMap := make(map[string]DBPollResult)
	for _, result := range dbPollResults {
		resultsMap[result.EndpointID] = result
	}

	circuitBreakerMap := make(map[string]CircuitBreaker, len(resultsMap))

	for _, breaker := range circuitBreakers {
		result := resultsMap[breaker.EndpointID]

		breaker.Requests = result.Successes + result.Failures
		breaker.TotalFailures = result.Failures
		breaker.TotalSuccesses = result.Successes
		breaker.FailureRate = float64(breaker.TotalFailures) / float64(breaker.TotalSuccesses+breaker.TotalFailures)

		// todo(raymond): move this to a different place that runs in a goroutine
		if breaker.State == StateOpen && cb.clock.Now().After(breaker.WillResetAt) {
			breaker.toHalfOpen()
		}

		if breaker.State == StateHalfOpen && breaker.TotalSuccesses >= cb.config.SuccessThreshold {
			breaker.resetCircuitBreaker()
		} else if breaker.State == StateClosed && (breaker.FailureRate >= cb.config.FailureThreshold || breaker.TotalFailures >= cb.config.FailureCount) {
			breaker.tripCircuitBreaker(cb.clock.Now().Add(time.Duration(cb.config.ErrorTimeout) * time.Second))
		}

		circuitBreakerMap[breaker.EndpointID] = breaker
	}

	// Update the circuit breaker state in Redis
	if err = cb.updateCircuitBreakersInRedis(ctx, circuitBreakerMap); err != nil {
		log.WithError(err).Error("Failed to update state in Redis")
	}

	return nil
}

// todo(raymond): move this to the delivery attempts repo
func (cb *CircuitBreakerManager) getFailureAndSuccessCounts(ctx context.Context, lookBackDuration int) (results []DBPollResult, err error) {
	query := `
		SELECT
            endpoint_id,
            COUNT(CASE WHEN status = false THEN 1 END) AS failures,
            COUNT(CASE WHEN status = true THEN 1 END) AS successes
        FROM convoy.delivery_attempts
        WHERE created_at >= NOW() - MAKE_INTERVAL(mins := $1) group by endpoint_id;
	`

	rows, err := cb.db.QueryxContext(ctx, query, lookBackDuration)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rowValue DBPollResult
		if rowScanErr := rows.StructScan(&rowValue); rowScanErr != nil {
			return nil, rowScanErr
		}
		results = append(results, rowValue)
	}

	return results, nil
}

func (cb *CircuitBreakerManager) updateCircuitBreakersInRedis(ctx context.Context, breakers map[string]CircuitBreaker) error {
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

	return nil
}

func (cb *CircuitBreakerManager) loadCircuitBreakerStateFromRedis(ctx context.Context) ([]CircuitBreaker, error) {
	keys, err := cb.redis.Keys(ctx, "breaker*").Result()
	if err != nil {
		return nil, err
	}

	res, err := cb.redis.MGet(ctx, keys...).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	var circuitBreakers []CircuitBreaker
	for i := range res {
		c := CircuitBreaker{}
		asBytes := []byte(res[i].(string))
		innerErr := msgpack.DecodeMsgPack(asBytes, &c)
		if innerErr != nil {
			return nil, innerErr
		}

		circuitBreakers = append(circuitBreakers, c)
	}

	return circuitBreakers, nil
}

func (cb *CircuitBreakerManager) Run(ctx context.Context) {
	for {
		// Get the failure and success counts from the last X minutes
		dbPollResults, err := cb.getFailureAndSuccessCounts(ctx, cb.config.ObservabilityWindow)
		if err != nil {
			log.WithError(err).Error("poll db failed")
		}

		if len(dbPollResults) == 0 {
			// there's nothing to update
			continue
		}

		err = cb.sampleEventsAndUpdateState(ctx, dbPollResults)
		if err != nil {
			log.WithError(err).Error("Failed to sample events and update state")
		}
		time.Sleep(time.Duration(cb.config.SampleTime) * time.Second)
	}
}
