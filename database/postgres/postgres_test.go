//go:build integration
// +build integration

package postgres

import (
	"os"
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

func getDB(t *testing.T) (database.Database, func()) {
	db, err := NewDB(getConfig())
	require.NoError(t, err)

	return db, func() {
		require.NoError(t, db.truncateTables())
	}
}
