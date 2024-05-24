//go:build integration
// +build integration

package postgres

import (
	"context"
	"fmt"
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
	_ = os.Setenv("CONVOY_DB_HOST", "0.0.0.0")
	_ = os.Setenv("CONVOY_REDIS_SCHEME", "redis")
	_ = os.Setenv("CONVOY_REDIS_PORT", "6379")

	_ = os.Setenv("CONVOY_DB_HOST", "localhost")
	_ = os.Setenv("CONVOY_DB_SCHEME", "postgres")
	_ = os.Setenv("CONVOY_DB_USERNAME", "admin")
	_ = os.Setenv("CONVOY_DB_PASSWORD", "password")
	_ = os.Setenv("CONVOY_DB_DATABASE", "convoy")
	_ = os.Setenv("CONVOY_DB_OPTIONS", "&sslmode=disable")
	_ = os.Setenv("CONVOY_DB_PORT", "5433")

	err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Get()
	if err != nil {
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
