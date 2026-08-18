package broker

import (
	"net"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// PatchTestConfig points cfg at the containers a test actually uses. Without
// this, cfg still describes localhost defaults and broker-built clients (cache,
// rate limiter, circuit breaker store) connect somewhere other than the
// testcontainer handles the test holds.
func PatchTestConfig(cfg *config.Configuration, conn *pgxpool.Pool, rd redis.UniversalClient) {
	if conn != nil {
		cfg.Database.DSN = conn.Config().ConnString()
	}
	if rd != nil {
		if host, port, err := splitHostPort(redisAddr(rd)); err == nil {
			cfg.Redis.Scheme = config.RedisScheme
			cfg.Redis.Host = host
			cfg.Redis.Port = port
			// BuildDsn prefers Addresses over Host/Port; drop stale cluster
			// config so broker clients connect to the testcontainer.
			cfg.Redis.Addresses = ""
		}
	}
}

func redisAddr(rd redis.UniversalClient) string {
	type optionsGetter interface {
		Options() *redis.Options
	}
	if c, ok := rd.(optionsGetter); ok {
		return c.Options().Addr
	}
	return ""
}

func splitHostPort(addr string) (string, int, error) {
	host, rawPort, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

// NewTest builds production broker dependencies for integration and E2E
// tests. Pass conn and rd so cfg is patched to the testcontainers under test.
// Call Close via t.Cleanup so per-test builds do not leak pools.
func NewTest(t *testing.T, cfg config.Configuration, db *sqlx.DB, logger log.Logger, conn *pgxpool.Pool, rd redis.UniversalClient) *Dependencies {
	t.Helper()

	PatchTestConfig(&cfg, conn, rd)

	deps, err := New(cfg, db, logger)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = deps.Close()
	})

	return deps
}
