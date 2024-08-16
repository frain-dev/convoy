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

type CircuitBreaker struct {
	State                State     `json:"state"`
	Requests             float64   `json:"requests"`
	EndpointID           string    `json:"endpoint_id"`
	TotalFailures        float64   `json:"total_failures"`
	TotalSuccesses       float64   `json:"total_successes"`
	WillResetAt          time.Time `json:"will_reset_at"`
	ConsecutiveFailures  float64   `json:"consecutive_failures"`
	ConsecutiveSuccesses float64   `json:"consecutive_successes"`
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

type DBPollResult struct {
	EndpointID string  `json:"endpoint_id" db:"endpoint_id"`
	Failures   float64 `json:"failures" db:"failures"`
	Successes  float64 `json:"successes" db:"successes"`
	FailRate   float64 `json:"fail_rate"`
}

func (pr *DBPollResult) CalculateFailRate() {
	pr.FailRate = 1 + (pr.Failures / (pr.Successes + pr.Failures))
}

type CircuitBreakerManager struct {
	breakers []CircuitBreaker
	clock    clock.Clock
	redis    *redis.Client
	db       *sqlx.DB
}

func NewCircuitBreakerManager(client redis.UniversalClient, db *sqlx.DB, clock clock.Clock) *CircuitBreakerManager {
	// todo(raymond): define and load breaker config
	r := &CircuitBreakerManager{
		db:    db,
		clock: clock,
		redis: client.(*redis.Client),
	}

	return r
}

func (cb *CircuitBreakerManager) sampleEventsAndUpdateState(ctx context.Context, dbPollResults []DBPollResult) error {
	for i := range dbPollResults {
		dbPollResults[i].FailRate = dbPollResults[i].Failures / (dbPollResults[i].Successes + dbPollResults[i].Failures)
	}

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
				State:       StateClosed,
				EndpointID:  dbPollResults[i].EndpointID,
				WillResetAt: cb.clock.Now().Add(time.Second * 30),
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
		// todo(raymond): these should be part of the config
		// 10% of total failed requests in the observability window
		threshold := 0.1
		// total number of failed requests in the observability window
		failureCount := 1.0

		result := resultsMap[breaker.EndpointID]
		fmt.Printf("result: %+v\n", result)

		// Apply the logic to decide whether to trip the breaker
		if result.FailRate > threshold || result.Failures >= failureCount {
			breaker.tripCircuitBreaker(cb.clock.Now().Add(time.Second * 30))
		} else if breaker.State == StateHalfOpen && result.Successes > 0 {
			breaker.resetCircuitBreaker()
		} else if breaker.State == StateOpen && cb.clock.Now().After(breaker.WillResetAt) {
			breaker.toHalfOpen()
		}

		breaker.Requests = result.Successes + result.Failures
		breaker.TotalFailures = result.Failures
		breaker.TotalSuccesses = result.Successes

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
	lookBackDuration := 5
	for {
		// Get the failure and success counts from the last X minutes
		dbPollResults, err := cb.getFailureAndSuccessCounts(ctx, lookBackDuration)
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
		time.Sleep(30 * time.Second)
	}
}
