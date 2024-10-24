package types

import (
	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/redis/go-redis/v9"
)

type ContextKey string

type APIOptions struct {
	FFlag    *fflag.FFlag
	DB       database.Database
	Redis    redis.UniversalClient
	Queue    queue.Queuer
	Logger   log.StdLogger
	Cache    cache.Cache
	Authz    *authz.Authz
	Rate     limiter.RateLimiter
	Licenser license.Licenser
	Cfg      config.Configuration
}
