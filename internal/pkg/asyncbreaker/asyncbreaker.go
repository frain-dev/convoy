package asyncbreaker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
)

type CircuitState string

const (
	OpenState     CircuitState = "open"
	ClosedState   CircuitState = "closed"
	HalfOpenState CircuitState = "half-open"
)

type endpoint struct {
	//Key             string       `json:"key" db:"key"`
	URL             string       `json:"url" redis:"url"`
	State           CircuitState `json:"state" redis:"state"`
	LastError       string       `json:"last_error" redis:"last_error"`
	Successes       uint         `json:"successes" redis:"successes"`
	CircuitOpenedAt time.Time    `json:"circuit_opened_at" redis:"circuit_opened_at"`
}

type endpointErrorRate struct {
	URL       string  `db:"url"`
	ErrorRate float32 `db:"error_rate"`
}

type endpoints map[string]*endpoint

const (
	BATCH_SIZE = 10_000

	// TODO(subomi): How do we account for scenarios where we have more endpoints?
	calculateEndpointErrorRateQuery = `
	WITH decoded_attempts AS (
    SELECT
        ed.endpoint_id,
        ed.created_at,
        jsonb_array_elements(convert_from(ed.attempts, 'utf8')::jsonb) AS da
    FROM
        event_deliveries ed
    WHERE 
		ed.created_at >= now() - $1::interval
    ),
    unnested_attempts AS (
    SELECT 
        endpoint_id,
        (da->>'status')::boolean AS status
    FROM 
        decoded_attempts
    )
    SELECT 
        endpoint_id,
        ROUND(COUNT(*) FILTER (WHERE status is null) * 100.0 / COUNT(*), 2) AS error_rate
    FROM 
        unnested_attempts
    GROUP BY 
        endpoint_id;
	`
	//retrieveEndpointStateQuery = `
	//select key, state, successes, circuit_opened_at, last_error from convoy.circuit_breaker Limit 100
	//`
)

type AsyncBreaker struct {
	db    *sqlx.DB
	store CircuitStore
	cfg   *config.CircuitBreakerConfiguration
}

func NewAsyncBreaker(db *sqlx.DB, store CircuitStore, cfg *config.CircuitBreakerConfiguration) (*AsyncBreaker, error) {
	return &AsyncBreaker{db: db, store: store, cfg: cfg}, nil
}

// Run executes an infinite loop to retrieve break endpoints
func (ab *AsyncBreaker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(ab.cfg.SampleTime) * time.Second)

	endpointMap := make(endpoints)
	for range ticker.C {
		processedEndpoints := make(map[string]struct{})

		err := ab.store.GetAllCircuits(ctx, endpointMap)
		if err != nil {
			log.WithError(err).Error("failed to refresh local endpoint state")
			continue
		}

		duration := time.Duration(ab.cfg.Duration) * time.Minute
		rows, err := ab.db.QueryxContext(ctx, calculateEndpointErrorRateQuery, duration.String())
		if err != nil {
			log.WithError(err).Error("failed to query endpoint state")
			continue
		}

		for rows.Next() {
			var er endpointErrorRate
			err = rows.StructScan(&er)
			if err != nil {
				log.WithError(err).Errorf("failed to scan endpoint error rate")
				continue
			}

			fmt.Printf("endpoint: %s, error_rate: %f\n", er.URL, er.ErrorRate)
			processedEndpoints[er.URL] = struct{}{}

			e, ok := endpointMap[er.URL]
			if !ok {
				// initialise endpoint in redis
				e := &endpoint{URL: er.URL}
				err = ab.store.UpsertCircuit(ctx, buildKey(er.URL).String(), e)
				if err != nil {
					log.WithError(err).Error("failed to update circuit state")
					continue
				}
			}

			err = ab.transitionEndpointState(ctx, e, er.ErrorRate)
			if err != nil {
				log.WithError(err).Errorf("failed to fully transition endpoint state")
				continue
			}
		}

		for k, e := range endpointMap {
			if _, ok := processedEndpoints[k]; !ok {
				continue
			}

			err = ab.transitionEndpointState(ctx, e, 0.0)
			if err != nil {
				log.WithError(err).Errorf("failed to fully transition endpoint state")
				continue
			}
		}
	}
}

//func (ab *AsyncBreaker) retrieveEndpointState(ctx context.Context) (endpoints, error) {
//	endpointMap := make(endpoints)
//
//	rows, err := ab.db.QueryxContext(ctx, retrieveEndpointStateQuery)
//	if err != nil {
//		return endpointMap, err
//	}
//
//	var e endpoint
//	for rows.Next() {
// add it to map
//		err = rows.StructScan(&e)
//		if err != nil {
//			log.WithError(err).Errorf("failed to scan endpoint breaker")
//			continue
//		}
//
//		endpointMap[e.Key] = e
//	}
//
//	return endpointMap, nil
//}

//func (ab *AsyncBreaker) retrieveEndpointErrorRate(ctx context.Context, endpointMap endpoints) error {
//	rows, err := ab.db.QueryxContext(ctx, calculateEndpointErrorRateQuery)
//	if err != nil {
//		log.WithError(err).Error("failed to query endpoint state")
//		return err
//	}
//
//	for rows.Next() {
//		var er endpointErrorRate
//		err = rows.StructScan(&er)
//		if err != nil {
//			log.WithError(err).Errorf("failed to scan endpoint error rate")
//			continue
//		}
//		if _, ok := endpointMap[er.URL]; !ok {
// insert new record.
//			query :=
//				`
//			insert into convoy.circuit_breaker (key, state)
//			values ($1, 'closed')
//			`
//			_, err = ab.db.ExecContext(ctx, query, er.URL)
//			if err != nil {
//				log.WithError(err).Errorf("failed to insert endpoint breaker - %s", er.URL)
//				continue
//			}
//
//			endpointMap[er.URL] = &endpoint{
//				URL:       er.URL,
//				State:     ClosedState,
//				Successes: 0,
//				ErrorRate: er.ErrorRate,
//			}
//
//			continue
//		}
//
//		endpoint := endpointMap[er.URL]
//		endpoint.ErrorRate = er.ErrorRate
//	}
//	return nil
//}

func (ab *AsyncBreaker) transitionEndpointState(ctx context.Context, endpoint *endpoint, error_rate float32) error {
	switch endpoint.State {
	case ClosedState:
		fmt.Println("closed state")
		if error_rate > float32(ab.cfg.ErrorThreshold) {
			fmt.Println("switching to the open state")
			endpoint.State = OpenState
			endpoint.Successes = 0
			endpoint.CircuitOpenedAt = time.Now()
			endpoint.LastError = ""
			err := ab.store.UpsertCircuit(ctx, buildKey(endpoint.URL).String(), endpoint)
			if err != nil {
				return err
			}
		}
		return nil
	case OpenState:
		openedAt := endpoint.CircuitOpenedAt
		if time.Since(openedAt) > time.Duration(ab.cfg.ErrorTimeout) {
			fmt.Println("switching to the half open state")
			endpoint.State = HalfOpenState
			endpoint.CircuitOpenedAt = time.Time{}
			err := ab.store.UpsertCircuit(ctx, buildKey(endpoint.URL).String(), endpoint)
			if err != nil {
				return err
			}
		}

		fmt.Println("Otherwise continue in the open state")
	case HalfOpenState:
		if !util.IsStringEmpty(endpoint.LastError) {
			fmt.Println("switching back to the open state")
			endpoint.State = OpenState
			endpoint.CircuitOpenedAt = time.Now()
			endpoint.Successes = 0
			endpoint.LastError = ""
			err := ab.store.UpsertCircuit(ctx, buildKey(endpoint.URL).String(), endpoint)
			if err != nil {
				return err
			}
		}

		if endpoint.Successes > ab.cfg.SuccessThreshold {
			fmt.Println("switching to the closed state")
			endpoint.State = ClosedState
			endpoint.Successes = 0
			err := ab.store.UpsertCircuit(ctx, buildKey(endpoint.URL).String(), endpoint)
			if err != nil {
				return err
			}
		}

		fmt.Println("Otherwise continue in the half open state")
	}

	return nil
}

var ErrCircuitOpen error = errors.New("circuit is open")

type Breaker interface {
	Run(ctx context.Context, key string, fn func() error) error
}

type CircuitStore interface {
	GetCircuit(ctx context.Context, key string, output *endpoint) error
	GetAllCircuits(ctx context.Context, output endpoints) error
	UpsertCircuit(ctx context.Context, key string, input *endpoint) error
	//ResetCircuit(ctx context.Context, key string) error
	IncrementSuccess(ctx context.Context, key string) error
}

const (
	namespace  = "convoy"
	delimiter  = ":"
	breakerKey = "circuit_breaker"
)

type redisStore struct {
	client *rdb.Redis
}

// TODO(subomi): add an extra namespace to avoid cross-project collision.
func NewRedisStore(client *rdb.Redis) *redisStore {
	return &redisStore{client: client}
}

func (rs *redisStore) GetCircuit(ctx context.Context, key string, e *endpoint) error {
	c := rs.client.Client()
	cmd := c.HGetAll(ctx, buildKey(key).String())
	if cmd.Err() != nil {
		return cmd.Err()
	}

	err := cmd.Scan(e)
	if err != nil {
		return err
	}

	return nil
}

func (rs *redisStore) GetAllCircuits(ctx context.Context, output endpoints) error {
	c := rs.client.Client()
	var cursor uint64
	var keys []string
	for {
		key := buildKey("*").String()
		cmd := c.Scan(ctx, cursor, key, BATCH_SIZE)
		if cmd.Err() != nil {
			return cmd.Err()
		}

		newKeys, cursor := cmd.Val()
		if len(newKeys) != 0 {
			keys = append(keys, newKeys...)
		}

		if cursor == 0 {
			break
		}
	}

	for _, key := range keys {
		e := &endpoint{}
		err := rs.GetCircuit(ctx, key, e)
		if err != nil {
			return err
		}

		output[e.URL] = e
	}

	return nil
}

func (rs *redisStore) UpsertCircuit(ctx context.Context, key string, e *endpoint) error {
	c := rs.client.Client()
	cmd := c.HSet(ctx, buildKey(key).String())
	if cmd.Err() != nil {
		return cmd.Err()
	}

	return nil
}

func (rs *redisStore) IncrementSuccess(ctx context.Context, key string) error {
	c := rs.client.Client()
	cmd := c.HIncrBy(ctx, key, "successes", 1)
	if cmd.Err() != nil {
		return cmd.Err()
	}

	return nil
}

type key string

func (k key) String() string {
	return string(k)
}

func buildKey(k string) key {
	var s strings.Builder

	s.WriteString(namespace)
	s.WriteString(delimiter)
	s.WriteString(breakerKey)
	s.WriteString(delimiter)
	s.WriteString(k)

	return key(s.String())
}

// EndpointBreaker is the actual breaker used in the dispatch flow
type EndpointBreaker struct {
	store       CircuitStore
	validErrors []error
	config      *config.CircuitBreakerConfiguration
}

func NewEndpointBreaker(config *config.CircuitBreakerConfiguration, errors ...error) *EndpointBreaker {
	return &EndpointBreaker{config: config, validErrors: errors}
}

func (eb *EndpointBreaker) Run(ctx context.Context, key string, fn func() error) error {
	var e endpoint

	//e, err := eb.retrieveEndpointState(ctx, key)
	err := eb.store.GetCircuit(ctx, key, &e)
	if err != nil {
		// fail open ?
		// log the error.
		return fn()
	}

	var state CircuitState
	if e == (endpoint{}) || util.IsStringEmpty(string(e.State)) {
		state = ClosedState
	} else {
		state = e.State
	}

	switch state {
	case ClosedState:
		err := fn()
		if err != nil {
			return err
		}
	case OpenState:
		return ErrCircuitOpen
	case HalfOpenState:
		// here we are waiting for the async breaker to catch up
		// and transition to the open state.
		if !util.IsStringEmpty(e.LastError) {
			return ErrCircuitOpen
		}

		err := fn()
		if err != nil {
			if isValidErr := eb.isValidError(err); isValidErr {
				e.Successes = 0
				e.LastError = err.Error()
				err = eb.store.UpsertCircuit(ctx, buildKey(key).String(), &e)
				return err
			}

			// TODO(subomi): Figure out the correct error to send here.
			return err
		}

		err = eb.store.IncrementSuccess(ctx, buildKey(key).String())
		if err != nil {
			return err
		}
	}

	return nil
}

// successes = 0, last_error = err
//func (eb *EndpointBreaker) updateCircuitWithError(ctx context.Context, cErr error) error {
//	query :=
//		`
//	update convoy.circuit_breaker
//	set successes = 0, last_error = $1
//	where key = $2;
//	`
//
//	_, err := eb.db.ExecContext(ctx, query)
//	if err != nil {
//		return err
//	}
//
//	return nil
//
//}

// successes += 1
//func (eb *EndpointBreaker) updateCircuitWithSuccess(ctx context.Context) error {
//	query :=
//		`
//		update convoy.circuit_breaker
//		set successes = successes + 1
//		where key = $1;
//	`
//
//	_, err := eb.db.ExecContext(ctx, query)
//	if err != nil {
//		return err
//	}
//
//	return nil
//
//}

// isValidError checks if the error is a valid endpoint failure error.
func (eb *EndpointBreaker) isValidError(rErr error) bool {
	for _, err := range eb.validErrors {
		if errors.Is(rErr, err) {
			return true
		}
	}

	return false
}
