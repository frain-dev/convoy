package bolt

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

func getDSN() string {
	return "database.db"
}

func getConfig() config.Configuration {
	return config.Configuration{
		Database: config.DatabaseConfiguration{
			Dsn: getDSN(),
		},
	}
}

func getDB(t *testing.T) (*bbolt.DB, func()) {
	db, err := New(getConfig())

	require.NoError(t, err)

	e := db.Client().(*bbolt.DB).Update(func(tx *bbolt.Tx) error {

		buckets := []string{"groups", "applications"}
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
