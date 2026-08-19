package pglock_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	dbpostgres "github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/pglock"
	"github.com/frain-dev/convoy/testenv"
)

var testInfra *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background(), testenv.WithoutRedis())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}
	testInfra = res
	code := m.Run()
	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup test infrastructure: %v\n", err)
	}
	os.Exit(code)
}

func TestTryLockExclusive(t *testing.T) {
	conn, err := testInfra.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)
	db := dbpostgres.NewFromConnection(conn).GetDB()
	ctx := context.Background()
	name := "convoy:pglock:test:" + ulid.Make().String()

	mu, err := pglock.TryLock(ctx, db, name)
	require.NoError(t, err)
	require.NotNil(t, mu)

	_, err = pglock.TryLock(ctx, db, name)
	require.ErrorIs(t, err, pglock.ErrNotObtained)

	require.NoError(t, mu.Unlock(ctx))

	mu2, err := pglock.TryLock(ctx, db, name)
	require.NoError(t, err)
	require.NoError(t, mu2.Unlock(ctx))
}
