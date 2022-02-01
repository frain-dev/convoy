package bolt

import (
	"context"
	"os"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/stretchr/testify/require"
	"github.com/timshannon/badgerhold/v4"
)

func getDSN() string {
	return os.Getenv("TEST_BOLT_DSN")
}

func getConfig() config.Configuration {
	return config.Configuration{
		Database: config.DatabaseConfiguration{
			Type: "bolt",
			Dsn:  getDSN(),
		},
	}
}

func getDB(t *testing.T) (*badgerhold.Store, func()) {
	db, err := New(getConfig())

	require.NoError(t, err)

	errr := os.Setenv("TZ", "") // Use UTC by default :)
	require.NoError(t, errr)

	return db.Client().(*badgerhold.Store), func() {
		require.NoError(t, db.Client().(*badgerhold.Store).Badger().DropAll())
		require.NoError(t, db.Disconnect(context.Background()))
	}
}
