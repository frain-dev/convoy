package pg

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
)

type PostgresRateLimiter struct {
	db database.Database
}

func NewRateLimiter(db database.Database) *PostgresRateLimiter {
	return &PostgresRateLimiter{db: db}
}

// TakeToken tries to take a token from the bucket.
//
// Creates the bucket if it doesn't exist and returns false if it is not successful.
// Returns true otherwise
func (p *PostgresRateLimiter) TakeToken(ctx context.Context, key string, rate int, bucketSize int) error {
	tx, err := p.db.GetDB().BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	var allowed bool
	err = tx.QueryRowContext(ctx, `select convoy.take_token($1, $2, $3)::bool;`, key, rate, bucketSize).Scan(&allowed)
	if err != nil {
		fmt.Printf("TakeToken: %+v\n", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		fmt.Printf("%+v\n", err)
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Fatalf("update failed: %v, unable to rollback: %v", err, rollbackErr)
		}
		return err
	}

	fmt.Printf(">>>>>>>>>>>>> %+v\n", allowed)
	if !allowed {
		return errors.New("rate limit error")
	}

	return nil
}
