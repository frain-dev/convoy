//go:build integration
// +build integration

package postgres

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"os"
	"sync"
	"testing"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/datastore"

	"github.com/stretchr/testify/require"
)

func getConfig() config.Configuration {
	_ = os.Setenv("CONVOY_DB_HOST", os.Getenv("TEST_REDIS_HOST"))
	_ = os.Setenv("CONVOY_REDIS_SCHEME", os.Getenv("TEST_REDIS_SCHEME"))
	_ = os.Setenv("CONVOY_REDIS_PORT", os.Getenv("TEST_REDIS_PORT"))

	_ = os.Setenv("CONVOY_DB_HOST", os.Getenv("TEST_DB_HOST"))
	_ = os.Setenv("CONVOY_DB_SCHEME", os.Getenv("TEST_DB_SCHEME"))
	_ = os.Setenv("CONVOY_DB_USERNAME", os.Getenv("TEST_DB_USERNAME"))
	_ = os.Setenv("CONVOY_DB_PASSWORD", os.Getenv("TEST_DB_PASSWORD"))
	_ = os.Setenv("CONVOY_DB_DATABASE", os.Getenv("TEST_DB_DATABASE"))
	_ = os.Setenv("CONVOY_DB_PORT", os.Getenv("TEST_DB_PORT"))

	_ = os.Setenv("CONVOY_LOCAL_ENCRYPTION_KEY", "test-key")

	err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Get()
	if err != nil {
		log.Fatal(err)
	}

	km, err := keys.NewLocalKeyManager()
	if err != nil {
		log.Fatal(err)
	}
	if km.IsSet() {
		if _, err = km.GetCurrentKey(); err != nil {
			log.Fatal(err)
		}
	}
	if err = keys.Set(km); err != nil {
		log.Fatal(err)
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
		dbHooks.RegisterHook(datastore.EndpointCreated, func(data interface{}, changelog interface{}) {})

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
