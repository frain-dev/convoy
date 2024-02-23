package limiter

import (
	"context"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/pkg/limiter/pg"
)

type RateLimiter interface {
	// Allow rate limits outgoing events to endpoints based on a rate in a specified time duration by the endpoint id
	Allow(ctx context.Context, key string, rate int, duration int) error
}

func NewLimiter(db database.Database) RateLimiter {
	ra := pg.NewRateLimiter(db)
	return ra
}
