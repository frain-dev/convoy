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
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/datastore"

	"github.com/stretchr/testify/require"
)

func getDSN() string {
	return os.Getenv("CONVOY_POSTGRES_DSN")
}

func getConfig() config.Configuration {
	return config.Configuration{
		Database: config.DatabaseConfiguration{
			Type:     config.PostgresDatabaseProvider,
			Host:     os.Getenv("CONVOY_DB_HOST"),
			Scheme:   os.Getenv("CONVOY_DB_SCHEME"),
			Username: os.Getenv("CONVOY_DB_USERNAME"),
			Password: os.Getenv("CONVOY_DB_PASSWORD"),
			Database: os.Getenv("CONVOY_DB_DATABASE"),
			Options:  os.Getenv("CONVOY_DB_OPTIONS"),
			Port:     5432,
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

		dbHooks := hooks.Init()
		dbHooks.RegisterHook(datastore.EndpointCreated, func(data interface{}) {})

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
