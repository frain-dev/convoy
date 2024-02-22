package limiter

import (
	"context"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/pkg/limiter/pg"
)

type RateLimiter interface {
	TakeToken(ctx context.Context, key string, rate int, bucketSize int) error
}

func NewLimiter(db database.Database) RateLimiter {
	ra := pg.NewRateLimiter(db)
	return ra
}
