package indexes

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestIsBlockedByData(t *testing.T) {
	require.False(t, IsBlockedByData(nil))
	require.False(t, IsBlockedByData(errors.New("lock timeout")))

	require.True(t, IsBlockedByData(&pgconn.PgError{Code: "23505"}))
	require.True(t, IsBlockedByData(errors.New("could not create unique index, key is duplicated")))
	require.True(t, IsBlockedByData(errors.New("duplicate key value violates unique constraint \"idx_test\"")))
}
