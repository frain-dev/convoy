//go:build integration

package broker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/worker/task"
)

func TestPostgresBrokerCloseReleasesDedicatedLockSessions(t *testing.T) {
	env := func(key, fallback string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return fallback
	}
	port, err := strconv.Atoi(env("TEST_DB_PORT", "5432"))
	require.NoError(t, err)
	dbConfig := config.DatabaseConfiguration{
		Scheme: "postgres", Host: env("TEST_DB_HOST", "localhost"),
		Username: env("TEST_DB_USERNAME", "convoy"), Password: env("TEST_DB_PASSWORD", "convoy"),
		Database: env("TEST_DB_DATABASE", "convoy"), Port: port,
		Options: "sslmode=disable", SetConnMaxLifetime: 3600,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dbConfig.BuildDsn())
	require.NoError(t, err)
	defer admin.Close(context.Background())
	name := fmt.Sprintf("broker_close_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	_, err = admin.Exec(ctx, "CREATE DATABASE "+quoted)
	require.NoError(t, err)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, dropErr := admin.Exec(cleanupCtx, "DROP DATABASE "+quoted)
		require.NoError(t, dropErr, "ordinary DROP must succeed after closing broker-owned sessions")
	}()

	dbConfig.Database = name
	appConfig, err := pgx.ParseConfig(dbConfig.BuildDsn())
	require.NoError(t, err)
	appDB := sqlx.NewDb(stdlib.OpenDB(*appConfig), "pgx")
	defer appDB.Close()
	cfg := testConfig(config.PostgresQueueProvider)
	cfg.Database = dbConfig
	logger := log.New("test", log.LevelError)
	deps, err := New(cfg, appDB, logger)
	require.NoError(t, err)
	defer deps.Close()

	otherDB, err := defaultOpenJobLockDB(dbConfig)
	require.NoError(t, err)
	defer otherDB.Close()
	other := newPostgresJobLockerWithLimit(otherDB.db, logger, jobLockMaxConns)
	// A competing session cannot enter while the callback holds the lock,
	// and can enter immediately after release.
	entered := false
	require.NoError(t, deps.JobLocker.WithLock(ctx, "broker-close-test", time.Second, func(context.Context) error {
		err := other.WithLock(ctx, "broker-close-test", time.Second, func(context.Context) error {
			entered = true
			return nil
		})
		require.ErrorIs(t, err, task.ErrLockBusy)
		return nil
	}))
	require.False(t, entered)
	require.NoError(t, other.WithLock(ctx, "broker-close-test", time.Second, func(context.Context) error {
		entered = true
		return nil
	}))
	require.True(t, entered)
	require.NoError(t, otherDB.Close())
	require.NoError(t, deps.Close())

	var one int
	require.NoError(t, appDB.QueryRowContext(ctx, "SELECT 1").Scan(&one), "broker must not close its borrowed application DB")
	require.Equal(t, 1, one)
	require.NoError(t, appDB.Close())
	require.Eventually(t, func() bool {
		var sessions int
		err := admin.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = $1", name).Scan(&sessions)
		return err == nil && sessions == 0
	}, 2*time.Second, 20*time.Millisecond, "broker Close must release all physical sessions")
}
