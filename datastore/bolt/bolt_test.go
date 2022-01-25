package bolt

import (
	"context"
	"os"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
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

func getDB(t *testing.T) (*bbolt.DB, func()) {
	db, err := New(getConfig())

	require.NoError(t, err)

	e := db.Client().(*bbolt.DB).Update(func(tx *bbolt.Tx) error {

		buckets := []string{"groups", "applications", "eventdeliveries", "apiKeys"}
		for _, v := range buckets {
			require.NoError(t, tx.DeleteBucket([]byte(v)))
		}

		return nil
	})

	require.NoError(t, e)

	return db.Client().(*bbolt.DB), func() {
		require.NoError(t, db.Disconnect(context.Background()))
	}
}
