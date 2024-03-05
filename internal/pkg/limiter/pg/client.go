package pg

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
)

type TokenBucketRateLimiter struct {
	db database.Database
}

func NewRateLimiter(db database.Database) *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{db: db}
}

func (p *TokenBucketRateLimiter) Allow(ctx context.Context, key string, rate int, bucketSize int) error {
	return p.takeToken(ctx, key, rate, bucketSize)
}

// TakeToken tries to take a token from the bucket.
//
// Creates the bucket if it doesn't exist and returns false if it is not successful.
// Returns true otherwise
func (p *TokenBucketRateLimiter) takeToken(ctx context.Context, key string, rate int, bucketSize int) error {
	// if one of rate and bucket size if zero, we skip processing
	if rate == 0 || bucketSize == 0 {
		return nil
	}

	tx, err := p.db.GetDB().BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	var allowed bool
	err = tx.QueryRowContext(ctx, `select convoy.take_token($1, $2, $3)::bool;`, key, rate, bucketSize).Scan(&allowed)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			log.Infof("update failed: %v, unable to rollback: %v", err, rollbackErr)
		}
		return err
	}

	if !allowed {
		return errors.New("rate limit error")
	}

	return nil
}
