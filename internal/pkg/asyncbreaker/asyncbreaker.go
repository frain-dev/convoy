package asyncbreaker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
	"gopkg.in/guregu/null.v4"
)

type CircuitState string

const (
	OpenState     CircuitState = "open"
	ClosedState   CircuitState = "closed"
	HalfOpenState CircuitState = "half-open"
)

type endpoint struct {
	Key             string       `json:"key" db:"key"`
	State           CircuitState `json:"state" db:"state"`
	LastError       string       `json:"last_error" db:"last_error"`
	Successes       uint         `json:"successes" db:"successes"`
	ErrorRate       float32      `json:"error_rate" db:"error_rate"`
	CircuitOpenedAt null.Time    `json:"circuit_opened_at" db:"circuit_opened_at,omitempty"`
}

type endpointErrorRate struct {
	EndpointID string  `db:"endpoint_id"`
	ErrorRate  float32 `db:"error_rate"`
}

type endpoints map[string]endpoint

const (
	calculateEndpointErrorRateQuery = `
	WITH decoded_attempts AS (
    SELECT
        ed.endpoint_id,
        ed.created_at,
        jsonb_array_elements(convert_from(ed.attempts, 'utf8')::jsonb) AS da
    FROM
        event_deliveries ed
    WHERE 
    	ed.created_at >= now() - interval '30 minutes'
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
	retrieveEndpointStateQuery = `
	select key, state, successes, circuit_opened_at, last_error from convoy.circuit_breaker Limit 100
	`
)

type AsyncBreaker struct {
	db  *sqlx.DB
	cfg *config.CircuitBreakerConfiguration
}

func NewAsyncBreaker(db *sqlx.DB, cfg *config.CircuitBreakerConfiguration) (*AsyncBreaker, error) {
	return &AsyncBreaker{db: db, cfg: cfg}, nil
}

// Run executes an infinite loop to retrieve break endpoints
func (ab *AsyncBreaker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(ab.cfg.SampleTime) * time.Second)

	for range ticker.C {
		endpoints, err := ab.retrieveEndpointState(ctx)
		if err != nil {
			log.WithError(err).Error("failed to refresh local endpoint state")
			continue
		}

		err = ab.retrieveEndpointErrorRate(ctx, endpoints)
		if err != nil {
			log.WithError(err).Error("failed to retrieve endpoint error rate")
			continue
		}

		//err = ab.transitionEndpointState(ctx, endpoints)
		//if err != nil {
		//	log.WithError(err).Errorf("failed to fully transition endpoint state")
		//	continue
		//}
	}
}

func (ab *AsyncBreaker) retrieveEndpointState(ctx context.Context) (endpoints, error) {
	endpointMap := make(endpoints)

	rows, err := ab.db.QueryxContext(ctx, retrieveEndpointStateQuery)
	if err != nil {
		return endpointMap, err
	}

	var e endpoint
	for rows.Next() {
		// add it to map
		err = rows.StructScan(&e)
		if err != nil {
			log.WithError(err).Errorf("failed to scan endpoint breaker")
			continue
		}

		endpointMap[e.Key] = e
	}

	return endpointMap, nil
}

func (ab *AsyncBreaker) retrieveEndpointErrorRate(ctx context.Context, endpointMap endpoints) error {
	rows, err := ab.db.QueryxContext(ctx, calculateEndpointErrorRateQuery)
	if err != nil {
		log.WithError(err).Error("failed to query endpoint state")
		return err
	}

	for rows.Next() {
		var er endpointErrorRate
		err = rows.StructScan(&er)
		if err != nil {
			log.WithError(err).Errorf("failed to scan endpoint error rate")
			continue
		}

		fmt.Printf("endpoint: %s, error_rate: %f\n", er.EndpointID, er.ErrorRate)
		if _, ok := endpointMap[er.EndpointID]; !ok {
			// insert new record.
			query :=
				`
			insert into convoy.circuit_breaker (key, state)
			values ($1, 'closed')
			`
			_, err = ab.db.ExecContext(ctx, query, er.EndpointID)
			if err != nil {
				log.WithError(err).Errorf("failed to insert endpoint breaker - %s", er.EndpointID)
				continue
			}

			endpointMap[er.EndpointID] = endpoint{
				Key:       er.EndpointID,
				State:     ClosedState,
				Successes: 0,
				ErrorRate: er.ErrorRate,
			}

			continue
		}

		endpoint := endpointMap[er.EndpointID]
		endpoint.ErrorRate = er.ErrorRate
	}
	return nil
}

func (ab *AsyncBreaker) transitionEndpointState(ctx context.Context, endpointMap endpoints) error {
	for _, endpoint := range endpointMap {
		switch endpoint.State {
		case ClosedState:
			fmt.Println("closed state")
			if endpoint.ErrorRate > float32(ab.cfg.ErrorThreshold) {
				fmt.Println("switching to the open state")
				query2 :=
					`
				update convoy.circuit_breaker
				set state = open, successes = 0, circuit_opened_at = now(), last_error = null
				where key = $1
				`

				_, err := ab.db.ExecContext(ctx, query2)
				if err != nil {
					log.WithError(err).Errorf("failed to update state for endpoint - %s", endpoint.Key)
					continue
				}
			}
			return nil
		case OpenState:
			openedAt := endpoint.CircuitOpenedAt.ValueOrZero()
			if time.Since(openedAt) > time.Duration(ab.cfg.ErrorTimeout) {
				fmt.Println("switching to the half open state")
				query :=
					`
				update convoy.circuit_breaker set circuit_opened_at = null, state = $1
				where key = $2
				`

				_, err := ab.db.ExecContext(ctx, query)
				if err != nil {
					log.WithError(err).Errorf("failed to update state for endpoint - %s", endpoint.Key)
					continue
				}
			}

			fmt.Println("Otherwise continue in the open state")
		case HalfOpenState:
			if !util.IsStringEmpty(endpoint.LastError) {
				fmt.Println("switching back to the open state")
				query :=
					`
				update convoy.circuit_breaker 
				set circuit_opened_at = now(), state = $1, successes = 0, last_error = null
				where key = $2`

				_, err := ab.db.ExecContext(ctx, query)
				if err != nil {
					log.WithError(err).Errorf("failed to update state for endpoint - %s", endpoint.Key)
					continue
				}
				return nil
			}

			if endpoint.Successes > ab.cfg.SuccessThreshold {
				fmt.Println("switching to the closed state")
				query :=
					`
				update convoy.circuit_breaker set state = $1, successes = 0
				where key = $2`

				_, err := ab.db.ExecContext(ctx, query)
				if err != nil {
					log.WithError(err).Errorf("failed to update state for endpoint - %s", endpoint.Key)
					continue
				}
			}

			fmt.Println("Otherwise continue in the half open state")
		}
	}

	return nil
}

var ErrCircuitOpen error = errors.New("circuit is open")

// EndpointBreaker is the actual breaker used in the dispatch flow
type EndpointBreaker struct {
	db       *sqlx.DB
	endpoint *endpoint
}

func (eb *EndpointBreaker) Run(ctx context.Context, fn func() error) error {
	var state CircuitState
	if eb.endpoint == nil {
		state = ClosedState
	} else {
		state = eb.endpoint.State
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
		if !util.IsStringEmpty(eb.endpoint.LastError) {
			return ErrCircuitOpen
		}

		err := fn()
		if err != nil {
			if isValidErr := eb.isValidError(err); isValidErr {
				err = eb.updateCircuitWithError(ctx, err)
				return err
			}

			// TODO(subomi): Figure out the correct error to send here.
			return err
		}

		err = eb.updateCircuitWithSuccess(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// successes = 0, last_error = err
func (eb *EndpointBreaker) updateCircuitWithError(ctx context.Context, cErr error) error {
	query :=
		`
	update convoy.circuit_breaker
	set successes = 0, last_error = $1
	where key = $2;
	`

	_, err := eb.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	return nil

}

// successes += 1
func (eb *EndpointBreaker) updateCircuitWithSuccess(ctx context.Context) error {
	query :=
		`
		update convoy.circuit_breaker
		set successes = successes + 1
		where key = $1;
	`

	_, err := eb.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	return nil

}

// isValidError checks if the error is a valid endpoint failure error.
func (eb *EndpointBreaker) isValidError(rErr error) bool { return true }
