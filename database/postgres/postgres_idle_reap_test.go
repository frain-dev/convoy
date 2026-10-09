//go:build integration

package postgres

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
)

// idleReapAppName isolates this test's backends from anything else using the
// same database, so the counts below describe only the pool under test.
const idleReapAppName = "convoy_idle_reap_test"

// idleReapDBConfig points at a throwaway Postgres. TEST_DB_* is the same
// convention the other integration tests in this package use.
func idleReapDBConfig(t *testing.T) config.DatabaseConfiguration {
	t.Helper()

	port := 5432
	if v := os.Getenv("TEST_DB_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		require.NoError(t, err)
		port = p
	}

	return config.DatabaseConfiguration{
		Scheme:   envOrDefault("TEST_DB_SCHEME", "postgres"),
		Host:     envOrDefault("TEST_DB_HOST", "localhost"),
		Username: envOrDefault("TEST_DB_USERNAME", "convoy"),
		Password: envOrDefault("TEST_DB_PASSWORD", "convoy"),
		Database: envOrDefault("TEST_DB_DATABASE", "convoy"),
		Port:     port,
		Options:  "sslmode=disable&application_name=" + idleReapAppName,
		// Carried from the compiled default in config.DefaultConfiguration.
		// Zero is not "no limit" to pgx: it sets every connection's max age to
		// its creation time, so the pool destroys each one on first use.
		SetConnMaxLifetime: 3600,
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// countBackends reports how many server-side connections this test's pool is
// holding. It runs on its own short-lived connection so it never borrows from
// the pool it is measuring, which would make an idle connection look busy.
func countBackends(t *testing.T, dbCfg config.DatabaseConfiguration) int {
	t.Helper()

	ctrlCfg := dbCfg
	ctrlCfg.Options = "sslmode=disable&application_name=" + idleReapAppName + "_ctrl"

	pool, err := pgxpool.New(context.Background(), ctrlCfg.BuildDsn())
	require.NoError(t, err)
	defer pool.Close()

	var n int
	err = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_stat_activity WHERE application_name = $1`,
		idleReapAppName).Scan(&n)
	require.NoError(t, err)
	return n
}

// waitForBackends polls until the pool has shed connections down to want, and
// reports the last observed count on failure so a timeout says how far it got
// rather than only that it expired.
func waitForBackends(t *testing.T, dbCfg config.DatabaseConfiguration, want int, timeout time.Duration) int {
	t.Helper()

	deadline := time.Now().Add(timeout)
	last := -1
	for time.Now().Before(deadline) {
		last = countBackends(t, dbCfg)
		if last <= want {
			return last
		}
		time.Sleep(250 * time.Millisecond)
	}

	t.Fatalf("pool still holding %d backends after %s, wanted <= %d", last, timeout, want)
	return last
}

// TestPoolReapsIdleConnections proves the pool actually closes idle
// connections, rather than only that MaxConnIdleTime was assigned. It builds the
// pool through buildPoolConfig, the same path parseDBConfig uses, so the tracer,
// notice handler and bounds under test are the shipped ones. Only the two
// durations that decide how long reaping takes are compressed, because the
// shipped MaxConnIdleTime is five minutes and the pgx health check that enforces
// it runs once a minute.
func TestPoolReapsIdleConnections(t *testing.T) {
	dbCfg := idleReapDBConfig(t)
	dbCfg.SetMaxOpenConnections = 20
	// Left unset on purpose: this is the configuration that shipped broken, and
	// the one the QA stack runs.
	dbCfg.SetMaxIdleConnections = 0

	pgxCfg, _, err := buildPoolConfig(dbCfg, &captureLogger{})
	require.NoError(t, err)
	require.Equal(t, maxConnIdleTime, pgxCfg.MaxConnIdleTime)

	pgxCfg.MaxConnIdleTime = 2 * time.Second
	pgxCfg.HealthCheckPeriod = 500 * time.Millisecond

	pool, err := pgxpool.NewWithConfig(context.Background(), pgxCfg)
	require.NoError(t, err)
	defer pool.Close()

	require.Equal(t, 0, countBackends(t, dbCfg), "database must be quiet before the test opens anything")

	const want = 15
	expandPool(t, pool, want)

	peak := countBackends(t, dbCfg)
	require.GreaterOrEqual(t, peak, want, "pool should have opened the connections the test asked for")

	got := waitForBackends(t, dbCfg, 0, 30*time.Second)
	require.Zero(t, got, "every idle connection should have been reaped")
	t.Logf("pgx pool: %d backends at peak, %d after idle reaping", peak, got)
}

// TestSqlxWrapperReapsIdleConnections covers the layer application code queries
// through, which is the one that decides whether the fix actually returns slots
// to the server. database/sql keeps its own free list of connections it has
// already acquired from pgx, and pgx counts an acquired connection as in use, so
// MaxConnIdleTime cannot reap anything sitting on that free list. The bound that
// matters there is SetMaxIdleConns: connections beyond it are released back to
// pgx as soon as a query finishes, and pgx then reaps them on the idle timeout.
// This test pins how far the pool actually drains, so a later change to either
// bound cannot quietly strand connections again.
func TestSqlxWrapperReapsIdleConnections(t *testing.T) {
	dbCfg := idleReapDBConfig(t)
	dbCfg.SetMaxOpenConnections = 20
	dbCfg.SetMaxIdleConnections = 0

	pgxCfg, _, err := buildPoolConfig(dbCfg, &captureLogger{})
	require.NoError(t, err)

	pgxCfg.MaxConnIdleTime = 2 * time.Second
	pgxCfg.HealthCheckPeriod = 500 * time.Millisecond

	pool, err := pgxpool.NewWithConfig(context.Background(), pgxCfg)
	require.NoError(t, err)
	defer pool.Close()

	db := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	defer func() {
		require.NoError(t, db.Close())
	}()

	// Exactly what parseDBConfig applies to the wrapper, so the numbers below
	// describe the shipped configuration and not a tuned-for-green one.
	maxIdle := idleConnsFor(dbCfg.SetMaxOpenConnections)
	db.SetMaxOpenConns(dbCfg.SetMaxOpenConnections)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(time.Second * time.Duration(dbCfg.SetConnMaxLifetime))

	require.Equal(t, 0, countBackends(t, dbCfg), "database must be quiet before the test opens anything")

	const want = 15
	expandSqlxPool(t, db, want)

	peak := countBackends(t, dbCfg)
	require.GreaterOrEqual(t, peak, want)

	// The free list is what remains: pgx reaps everything the wrapper hands
	// back, and holds the rest until SetConnMaxLifetime retires it.
	got := waitForBackends(t, dbCfg, maxIdle, 30*time.Second)
	require.LessOrEqual(t, got, maxIdle,
		"everything above the sqlx free list should have been released to pgx and reaped")
	t.Logf("sqlx wrapper (max idle %d): %d backends at peak, %d after idle reaping", maxIdle, peak, got)
}

// expandPool forces the pool to open n distinct connections by holding every
// acquisition until the last one is in hand.
func expandPool(t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()

	ctx := context.Background()
	conns := make([]*pgxpool.Conn, 0, n)
	for i := 0; i < n; i++ {
		c, err := pool.Acquire(ctx)
		require.NoError(t, err)

		var one int
		require.NoError(t, c.QueryRow(ctx, "SELECT 1").Scan(&one))
		conns = append(conns, c)
	}
	for _, c := range conns {
		c.Release()
	}
}

// expandSqlxPool does the same through database/sql. The queries run
// concurrently because database/sql hands a single sequential caller the same
// connection every time and the pool would never grow.
func expandSqlxPool(t *testing.T, db *sqlx.DB, n int) {
	t.Helper()

	var wg sync.WaitGroup
	release := make(chan struct{})
	errs := make(chan error, n)

	// Unblocks the holders on every exit, including the assertion failure
	// below, which leaves the test goroutine without running the rest of this
	// function.
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	defer func() {
		releaseAll()
		wg.Wait()
	}()

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := db.Connx(context.Background())
			if err != nil {
				errs <- err
				return
			}
			defer func() { _ = conn.Close() }()

			var one int
			if err = conn.QueryRowxContext(context.Background(), "SELECT 1").Scan(&one); err != nil {
				errs <- err
				return
			}
			<-release
		}()
	}

	// Wait for every goroutine to be holding its own connection. This polls in
	// the test goroutine rather than through require.Eventually, whose
	// condition runs on its own goroutine where a failed assertion cannot fail
	// the test. A stall here is usually a connection error rather than a slow
	// pool, so surface that error: "never reached 15 open connections"
	// describes the symptom and hides the cause.
	deadline := time.Now().Add(20 * time.Second)
	for db.Stats().OpenConnections < n {
		select {
		case err := <-errs:
			require.NoError(t, err, "opening a connection failed while expanding the pool")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("sqlx pool reached only %d of %d open connections", db.Stats().OpenConnections, n)
		}
		time.Sleep(100 * time.Millisecond)
	}

	releaseAll()
	wg.Wait()

	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

// idleConnsFor mirrors the sqlx idle bound parseDBConfig derives when
// max_idle_conn is unset, so the test exercises the shipped ratio.
func idleConnsFor(maxOpen int) int {
	idle := maxOpen / 4
	if idle < 2 {
		idle = 2
	}
	return idle
}
