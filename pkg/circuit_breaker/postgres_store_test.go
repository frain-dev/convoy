package circuit_breaker_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	dbpostgres "github.com/frain-dev/convoy/database/postgres"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/testenv"
)

func TestPostgresStore(t *testing.T) {
	res, cleanup, err := testenv.Launch(context.Background(), testenv.WithoutRedis())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup() })

	conn, err := res.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)
	store := cb.NewPostgresStore(dbpostgres.NewFromConnection(conn).GetDB())
	ctx := context.Background()
	key := "breaker:" + ulid.Make().String()

	_, err = store.GetOne(ctx, key)
	require.ErrorIs(t, err, cb.ErrCircuitBreakerNotFound)

	require.NoError(t, store.SetOne(ctx, key, "payload", time.Minute))
	got, err := store.GetOne(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "payload", got)

	many, err := store.GetMany(ctx, key, key+"missing")
	require.NoError(t, err)
	require.Equal(t, "payload", many[0])
	require.Nil(t, many[1])

	keys, err := store.Keys(ctx, "breaker:")
	require.NoError(t, err)
	require.Contains(t, keys, key)

	mu, err := store.Lock(ctx, "convoy:cb:test:"+ulid.Make().String(), 30)
	require.NoError(t, err)
	require.NoError(t, store.Unlock(ctx, mu))
}
