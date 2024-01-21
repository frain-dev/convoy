package types

import (
	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
)

type ContextKey string

type APIOptions struct {
	FFlag  *fflag.FFlag
	DB     database.Database
	Queue  queue.Queuer
	Logger log.StdLogger
	Cache  cache.Cache
	Authz  *authz.Authz
}
