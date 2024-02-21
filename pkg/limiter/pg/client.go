package pg

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/database"
)

type PgRateLimiter struct {
	db database.Database
}

func NewRateLimiter(db database.Database) *PgRateLimiter {
	return &PgRateLimiter{db: db}
}

func (p *PgRateLimiter) TakeToken(ctx context.Context, key string, limit int) error {
	tx, err := p.db.GetDB().BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var allowed bool
	err = tx.QueryRowContext(ctx, `select convoy.take_token($1, $2)::bool;`, key, limit).Scan(&allowed)
	if err != nil {
		fmt.Printf("TakeToken: %+v\n", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return err
	}

	fmt.Printf(">>>>>>>>>>>>> %+v\n", allowed)
	if !allowed {
		return errors.New("rate limit error")
	}

	return nil
}
