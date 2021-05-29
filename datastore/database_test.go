// +build integration

package datastore

import (
	"context"
	"os"
	"testing"

	"github.com/hookcamp/hookcamp/config"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
)

func getDSN() string {
	return os.Getenv("TEST_DATABASE_DSN")
}

func getConfig() config.Configuration {

	return config.Configuration{
		Database: config.DatabaseConfiguration{
			Dsn: getDSN(),
		},
	}
}

func getDB(t *testing.T) (*mongo.Database, func()) {

	db, err := New(getConfig())
	require.NoError(t, err)

	return db.Database("hookcamp", nil), func() {
		require.NoError(t, db.Disconnect(context.Background()))
	}
}
