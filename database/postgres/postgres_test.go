//go:build integration

package postgres

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/keys"
)

// fromTestEnv copies a TEST_* value onto the CONVOY_* name the configuration
// reads. An unset or empty source is left alone rather than exported empty,
// because envconfig treats a set-but-empty variable as a value and would
// overwrite the compiled default with it, turning "I did not configure this" into
// an empty DSN.
func fromTestEnv(convoyKey, testKey string) {
	if v := os.Getenv(testKey); v != "" {
		_ = os.Setenv(convoyKey, v)
	}
}

func getConfig() config.Configuration {
	fromTestEnv("CONVOY_REDIS_HOST", "TEST_REDIS_HOST")
	fromTestEnv("CONVOY_REDIS_SCHEME", "TEST_REDIS_SCHEME")
	fromTestEnv("CONVOY_REDIS_PORT", "TEST_REDIS_PORT")

	fromTestEnv("CONVOY_DB_HOST", "TEST_DB_HOST")
	fromTestEnv("CONVOY_DB_SCHEME", "TEST_DB_SCHEME")
	fromTestEnv("CONVOY_DB_USERNAME", "TEST_DB_USERNAME")
	fromTestEnv("CONVOY_DB_PASSWORD", "TEST_DB_PASSWORD")
	fromTestEnv("CONVOY_DB_DATABASE", "TEST_DB_DATABASE")
	fromTestEnv("CONVOY_DB_PORT", "TEST_DB_PORT")

	_ = os.Setenv("CONVOY_LOCAL_ENCRYPTION_KEY", "test-key")

	// The CONVOY_* variables above only reach the configuration through
	// envconfig, the same way cmd/hooks loads it. Without this override
	// LoadConfig returns the compiled defaults and every TEST_DB_* value is
	// silently ignored, which happens to work only when the local database
	// matches CI's postgres:postgres@localhost:5432/convoy.
	err := config.LoadConfig("", func(c *config.Configuration) error {
		return envconfig.Process("convoy", c)
	})
	if err != nil {
		panic(err)
	}

	cfg, err := config.Get()
	if err != nil {
		panic(err)
	}

	km, err := keys.NewLocalKeyManager("test")
	if err != nil {
		panic(err)
	}
	if km.IsSet() {
		if _, err = km.GetCurrentKeyFromCache(); err != nil {
			panic(err)
		}
	}
	if err = keys.Set(km); err != nil {
		panic(err)
	}

	return cfg
}

var (
	once = sync.Once{}
	_db  *Postgres
)

func getDB(t *testing.T) (database.Database, func()) {
	once.Do(func() {
		var err error

		dbHooks := hooks.Init()
		dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data interface{}, changelog interface{}) {})

		_db, err = NewDB(getConfig())
		require.NoError(t, err)
	})

	return _db, func() {
		require.NoError(t, _db.truncateTables())
	}
}

func (p *Postgres) truncateTables() error {
	tables := `
		convoy.event_deliveries,
		convoy.events,
		convoy.api_keys,
		convoy.subscriptions,
		convoy.source_verifiers,
		convoy.sources,
		convoy.configurations,
		convoy.devices,
		convoy.portal_links,
		convoy.organisation_invites,
		convoy.applications,
        convoy.endpoints,
		convoy.projects,
		convoy.project_configurations,
		convoy.organisation_members,
		convoy.organisations,
		convoy.users
	`

	_, err := p.dbx.ExecContext(context.Background(), fmt.Sprintf("TRUNCATE %s CASCADE;", tables))
	if err != nil {
		return err
	}

	return nil
}
