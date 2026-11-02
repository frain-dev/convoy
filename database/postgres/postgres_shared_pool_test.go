//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Closing SQL connections must return their slots to the shared pgx pool,
// including when the configured SQL idle limit could otherwise pin every slot.
func TestSQLWrapperReleasesSharedPoolConnections(t *testing.T) {
	for _, idle := range []int{0, -1, 2, 40} {
		t.Run(fmt.Sprintf("configured_idle_%d", idle), func(t *testing.T) {
			cfg := idleReapDBConfig(t)
			cfg.SetMaxOpenConnections = 2
			cfg.SetMaxIdleConnections = idle
			db, err := parseDBConfig(cfg, &captureLogger{})
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, db.GetDB().Close())
				db.GetConn().Close()
			})

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var sqlConns []*sql.Conn
			defer func() {
				for _, conn := range sqlConns {
					_ = conn.Close()
				}
			}()
			for i := 0; i < 2; i++ {
				conn, err := db.GetDB().Conn(ctx)
				require.NoError(t, err)
				sqlConns = append(sqlConns, conn)
				var one int
				require.NoError(t, conn.QueryRowContext(ctx, "SELECT 1").Scan(&one))
				require.Equal(t, 1, one)
			}
			require.EqualValues(t, 2, db.GetConn().Stat().AcquiredConns())
			for _, conn := range sqlConns {
				require.NoError(t, conn.Close())
			}
			sqlConns = nil

			acquireCtx, acquireCancel := context.WithTimeout(context.Background(), time.Second)
			defer acquireCancel()
			var nativeConns []*pgxpool.Conn
			defer func() {
				for _, conn := range nativeConns {
					conn.Release()
				}
			}()
			for i := 0; i < 2; i++ {
				conn, err := db.GetConn().Acquire(acquireCtx)
				require.NoError(t, err, "SQL wrapper must release every slot for native callers")
				nativeConns = append(nativeConns, conn)
				var one int
				require.NoError(t, conn.QueryRow(acquireCtx, "SELECT 1").Scan(&one))
				require.Equal(t, 1, one)
			}
		})
	}
}

// database/sql closes its wrapper after each operation, but pgx must reuse the
// physical connection instead of opening a new PostgreSQL session each time.
func TestSQLWrapperReusesPhysicalConnections(t *testing.T) {
	for _, lifetime := range []int{3600, 0, -1} {
		t.Run(fmt.Sprintf("lifetime_%d", lifetime), func(t *testing.T) {
			testSQLWrapperReusesPhysicalConnections(t, lifetime)
		})
	}
}

func testSQLWrapperReusesPhysicalConnections(t *testing.T, lifetime int) {
	t.Helper()
	cfg := idleReapDBConfig(t)
	cfg.SetMaxOpenConnections = 1
	cfg.SetMaxIdleConnections = 40
	cfg.SetConnMaxLifetime = lifetime
	db, err := parseDBConfig(cfg, &captureLogger{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var pid int
	require.NoError(t, db.GetDB().QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&pid))
	for i := 0; i < 100; i++ {
		var got int
		require.NoError(t, db.GetDB().QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&got))
		require.Equal(t, pid, got)

		tx, err := db.GetDB().BeginTx(ctx, nil)
		require.NoError(t, err)
		queryErr := tx.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&got)
		if queryErr != nil {
			_ = tx.Rollback()
			require.NoError(t, queryErr)
		}
		require.NoError(t, tx.Commit())
		require.Equal(t, pid, got)

		require.NoError(t, db.GetConn().QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&got))
		require.Equal(t, pid, got)
	}
	require.EqualValues(t, 1, db.GetConn().Stat().NewConnsCount(), "all operations must reuse one physical connection")
	require.Zero(t, db.GetDB().Stats().OpenConnections, "SQL wrapper should retain no idle connections")
	require.EqualValues(t, 0, db.GetConn().Stat().AcquiredConns())
	t.Logf("100 SQL queries, 100 SQL transactions and 100 native queries reused backend %d; physical connections created: %d", pid, db.GetConn().Stat().NewConnsCount())
}
