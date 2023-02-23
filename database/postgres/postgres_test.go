package postgres

import (
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/stretchr/testify/require"
)

func getDB(t *testing.T) (*sqlx.DB, func()) {
	db, err := NewDB()
	require.NoError(t, err)

	return db.dbx, func() {
		require.NoError(t, db.truncateTables())
	}
}
