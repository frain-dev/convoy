package postgres

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	dbpostgres "github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/testenv"
)

// namedData matches the Redis cache integration test shape.
type namedData struct {
	Name string
}

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

func setupCache(t *testing.T) *PostgresCache {
	t.Helper()
	conn, err := testInfra.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)
	return New(dbpostgres.NewFromConnection(conn).GetDB())
}

func TestSetGetDelete(t *testing.T) {
	c := setupCache(t)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	var got string
	require.NoError(t, c.Get(ctx, key, &got))
	require.Empty(t, got)

	require.NoError(t, c.Set(ctx, key, "hello", time.Minute))
	require.NoError(t, c.Get(ctx, key, &got))
	require.Equal(t, "hello", got)

	require.NoError(t, c.Delete(ctx, key))
	got = ""
	require.NoError(t, c.Get(ctx, key, &got))
	require.Empty(t, got)
}

func TestConsumeIsAtomic(t *testing.T) {
	c := setupCache(t)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()
	require.NoError(t, c.Set(ctx, key, "hello", time.Minute))

	start := make(chan struct{})
	found := make(chan bool, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			var got string
			ok, err := c.Consume(ctx, key, &got)
			require.NoError(t, err)
			if ok {
				require.Equal(t, "hello", got)
			}
			found <- ok
		}()
	}

	close(start)
	wg.Wait()
	close(found)

	consumed := 0
	for ok := range found {
		if ok {
			consumed++
		}
	}
	require.Equal(t, 1, consumed)
}

func TestGetExpiredIsMiss(t *testing.T) {
	c := setupCache(t)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	require.NoError(t, c.Set(ctx, key, "gone", time.Millisecond))
	time.Sleep(20 * time.Millisecond)

	var got string
	require.NoError(t, c.Get(ctx, key, &got))
	require.Empty(t, got)
}

func TestNamedDataRoundTrip(t *testing.T) {
	c := setupCache(t)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	require.NoError(t, c.Set(ctx, key, &namedData{Name: "test_name"}, time.Minute))

	var got namedData
	require.NoError(t, c.Get(ctx, key, &got))
	require.Equal(t, "test_name", got.Name)
}

func TestSubscriptionPreservesEndpointID(t *testing.T) {
	c := setupCache(t)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	sub := datastore.Subscription{
		UID:        "sub-1",
		Name:       "api-sub",
		ProjectID:  "proj-1",
		SourceID:   "src-1",
		EndpointID: "ep-1",
		DeviceID:   "dev-1",
	}
	require.NoError(t, c.Set(ctx, key, &sub, time.Minute))

	var got datastore.Subscription
	require.NoError(t, c.Get(ctx, key, &got))
	require.Equal(t, "sub-1", got.UID)
	require.Equal(t, "ep-1", got.EndpointID)
	require.Equal(t, "src-1", got.SourceID)
	require.Equal(t, "dev-1", got.DeviceID)
}

func TestEndpointPreservesJSONDashFields(t *testing.T) {
	c := setupCache(t)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	ep := datastore.Endpoint{
		UID:             "ep-1",
		ProjectID:       "proj-1",
		OwnerID:         "owner-1",
		AppID:           "app-1",
		OAuth2Config:    &datastore.OAuth2{ClientID: "cid"},
		BasicAuthConfig: &datastore.BasicAuth{UserName: "u", Password: "p"},
	}
	require.NoError(t, c.Set(ctx, key, &ep, time.Minute))

	var got datastore.Endpoint
	require.NoError(t, c.Get(ctx, key, &got))
	require.Equal(t, "ep-1", got.UID)
	require.Equal(t, "app-1", got.AppID)
	require.NotNil(t, got.OAuth2Config)
	require.Equal(t, "cid", got.OAuth2Config.ClientID)
	require.NotNil(t, got.BasicAuthConfig)
	require.Equal(t, "u", got.BasicAuthConfig.UserName)
}

func TestGetDecodeErrorIsMiss(t *testing.T) {
	c := setupCache(t)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	_, err := c.db.ExecContext(ctx, `
		INSERT INTO convoy.kv_cache (key, value, expires_at)
		VALUES ($1, $2, NULL)`,
		key, []byte(`{"uid":"sub-1","name":"api-sub"}`),
	)
	require.NoError(t, err)

	var got datastore.Subscription
	require.NoError(t, c.Get(ctx, key, &got))
	require.Empty(t, got.UID)
	require.Empty(t, got.EndpointID)
}

func TestGetStrictReturnsDecodeError(t *testing.T) {
	c := setupCache(t)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	_, err := c.db.ExecContext(ctx, `
		INSERT INTO convoy.kv_cache (key, value, expires_at)
		VALUES ($1, $2, NULL)`,
		key, []byte(`not-msgpack`),
	)
	require.NoError(t, err)

	var got bool
	err = c.GetStrict(ctx, key, &got)
	require.ErrorContains(t, err, "decode cache value")
	require.False(t, got)
}

func TestGetOrCreateBytes(t *testing.T) {
	c := setupCache(t)
	ctx := context.Background()
	key := "setnx:" + ulid.Make().String()

	first, err := c.GetOrCreateBytes(ctx, key, []byte("one"))
	require.NoError(t, err)
	require.Equal(t, []byte("one"), first)

	second, err := c.GetOrCreateBytes(ctx, key, []byte("two"))
	require.NoError(t, err)
	require.Equal(t, []byte("one"), second)
}

// setupLocalCache builds a cache that may answer Get from process memory.
func setupLocalCache(t *testing.T, ttl time.Duration) *PostgresCache {
	t.Helper()
	conn, err := testInfra.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)
	return NewWithLocalReads(dbpostgres.NewFromConnection(conn).GetDB(), ttl, 128)
}

// removeRow deletes straight from the table, bypassing the cache API so the
// in-process copy survives. A later Get that still returns the old value
// therefore proves the value came from memory rather than the database.
func removeRow(t *testing.T, c *PostgresCache, key string) {
	t.Helper()
	_, err := c.db.ExecContext(context.Background(), `DELETE FROM convoy.kv_cache WHERE key = $1`, key)
	require.NoError(t, err)
}

func TestLocalReadsAnswerRepeatsAndExpire(t *testing.T) {
	const ttl = 150 * time.Millisecond
	c := setupLocalCache(t, ttl)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	require.NoError(t, c.Set(ctx, key, "hello", time.Minute))

	var got string
	require.NoError(t, c.Get(ctx, key, &got))
	require.Equal(t, "hello", got)

	removeRow(t, c, key)

	got = ""
	require.NoError(t, c.Get(ctx, key, &got))
	require.Equal(t, "hello", got, "repeat read should come from memory, not the table")

	time.Sleep(ttl * 3)

	got = ""
	require.NoError(t, c.Get(ctx, key, &got))
	require.Empty(t, got, "the local copy must not outlive its ttl")
}

func TestLocalReadsDroppedByEveryWrite(t *testing.T) {
	ctx := context.Background()

	t.Run("set", func(t *testing.T) {
		c := setupLocalCache(t, time.Minute)
		key := "cache:" + ulid.Make().String()
		require.NoError(t, c.Set(ctx, key, "first", time.Minute))

		var got string
		require.NoError(t, c.Get(ctx, key, &got))
		require.Equal(t, "first", got)

		require.NoError(t, c.Set(ctx, key, "second", time.Minute))
		require.NoError(t, c.Get(ctx, key, &got))
		require.Equal(t, "second", got, "a writer must not read back its own stale value")
	})

	t.Run("delete", func(t *testing.T) {
		c := setupLocalCache(t, time.Minute)
		key := "cache:" + ulid.Make().String()
		require.NoError(t, c.Set(ctx, key, "value", time.Minute))

		var got string
		require.NoError(t, c.Get(ctx, key, &got))
		require.Equal(t, "value", got)

		require.NoError(t, c.Delete(ctx, key))
		got = ""
		require.NoError(t, c.Get(ctx, key, &got))
		require.Empty(t, got)
	})

	t.Run("consume", func(t *testing.T) {
		c := setupLocalCache(t, time.Minute)
		key := "cache:" + ulid.Make().String()
		require.NoError(t, c.Set(ctx, key, "once", time.Minute))

		var got string
		require.NoError(t, c.Get(ctx, key, &got))
		require.Equal(t, "once", got)

		var consumed string
		ok, err := c.Consume(ctx, key, &consumed)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "once", consumed)

		got = ""
		require.NoError(t, c.Get(ctx, key, &got))
		require.Empty(t, got, "a consumed one-shot value must not be observable again")
	})
}

func TestGetStrictIgnoresLocalReads(t *testing.T) {
	c := setupLocalCache(t, time.Minute)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	require.NoError(t, c.Set(ctx, key, true, time.Minute))

	var got bool
	require.NoError(t, c.Get(ctx, key, &got))
	require.True(t, got)

	removeRow(t, c, key)

	var strict bool
	require.NoError(t, c.GetStrict(ctx, key, &strict))
	require.False(t, strict, "authoritative reads must always go to the table")

	var loose bool
	require.NoError(t, c.Get(ctx, key, &loose))
	require.True(t, loose, "the local copy is still live, so Get should differ from GetStrict here")
}

func TestLocalReadsGiveEachCallerItsOwnValue(t *testing.T) {
	c := setupLocalCache(t, time.Minute)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	require.NoError(t, c.Set(ctx, key, []string{"a", "b"}, time.Minute))

	var first []string
	require.NoError(t, c.Get(ctx, key, &first))
	require.Equal(t, []string{"a", "b"}, first)

	first[0] = "mutated"

	var second []string
	require.NoError(t, c.Get(ctx, key, &second))
	require.Equal(t, []string{"a", "b"}, second, "one caller mutating its result must not corrupt another's")
}

func TestLocalReadsDisabled(t *testing.T) {
	c := setupCache(t)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	require.NoError(t, c.Set(ctx, key, "hello", time.Minute))

	var got string
	require.NoError(t, c.Get(ctx, key, &got))
	require.Equal(t, "hello", got)

	removeRow(t, c, key)

	got = ""
	require.NoError(t, c.Get(ctx, key, &got))
	require.Empty(t, got, "without local reads every Get must hit the table")
}

// TestLocalReadsNotResurrectedByConcurrentGet covers the interleaving the
// sequential write-then-Get tests cannot reach: a Get that has already read the
// old row from the table but has not yet stored it locally, racing a Delete that
// drops the local copy in between. Without a guard the reader's store lands
// after the writer's invalidation and the deleted value answers reads on this
// replica for the rest of the entry's lifetime, including for the writer.
//
// Landing inside that window depends on timing, so the loop widens the odds.
// The assertion is safe either way: a run that misses the window sees an empty
// value too, so this can only fail when the resurrection is real.
func TestLocalReadsNotResurrectedByConcurrentGet(t *testing.T) {
	c := setupLocalCache(t, time.Minute)
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		key := "cache:" + ulid.Make().String()
		require.NoError(t, c.Set(ctx, key, "v1", time.Minute))

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			var read string
			_ = c.Get(ctx, key, &read)
		}()

		time.Sleep(time.Duration(i%10) * 50 * time.Microsecond)
		require.NoError(t, c.Delete(ctx, key))
		wg.Wait()

		var after string
		require.NoError(t, c.Get(ctx, key, &after))
		require.Empty(t, after, "a deleted key must not be resurrected by an in-flight Get (iteration %d)", i)
	}
}

// TestLocalReadsFallThroughOnCorruptEntry covers bytes that will not decode.
// They must not keep answering from memory for the rest of the entry's life:
// the entry is dropped and the table decides.
func TestLocalReadsFallThroughOnCorruptEntry(t *testing.T) {
	c := setupLocalCache(t, time.Minute)
	ctx := context.Background()
	key := "cache:" + ulid.Make().String()

	require.NoError(t, c.Set(ctx, key, "good", time.Minute))
	c.local.Add(key, []byte{0xc1, 0xc1, 0xc1})

	var got string
	require.NoError(t, c.Get(ctx, key, &got))
	require.Equal(t, "good", got, "a corrupt local entry must fall through to the table")

	_, stillCached := c.local.Get(key)
	require.True(t, stillCached, "the table value should replace the corrupt entry")
}
