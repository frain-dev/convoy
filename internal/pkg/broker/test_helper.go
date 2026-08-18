package broker

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// NewTest builds production broker dependencies for integration and E2E
// tests. Call Close via t.Cleanup so per-test builds do not leak pools.
func NewTest(t *testing.T, cfg config.Configuration, db *sqlx.DB, logger log.Logger) *Dependencies {
	t.Helper()

	deps, err := New(cfg, db, logger)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = deps.Close()
	})

	return deps
}
