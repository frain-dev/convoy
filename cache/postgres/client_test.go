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
