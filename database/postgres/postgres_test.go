//go:build integration
// +build integration

package postgres

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"

	"github.com/stretchr/testify/require"
)

func getDSN() string {
	return os.Getenv("TEST_POSTGRES_DSN")
}

func getConfig() config.Configuration {
	return config.Configuration{
		Database: config.DatabaseConfiguration{
			Dsn: getDSN(),
		},
	}
}

var (
	once = sync.Once{}
	db   *Postgres
)

func getDB(t *testing.T) (database.Database, func()) {
	once.Do(func() {
		var err error
		db, err = NewDB(getConfig())
		require.NoError(t, err)
	})

	return db, func() {
		require.NoError(t, db.truncateTables())
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
