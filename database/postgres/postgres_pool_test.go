package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
)

func poolTestDBConfig() config.DatabaseConfiguration {
	return config.DatabaseConfiguration{
		Scheme:   "postgres",
		Host:     "localhost",
		Username: "convoy",
		Password: "convoy",
		Database: "convoy",
		Port:     5432,
		Options:  "sslmode=disable",
	}
}

// TestBuildPoolConfigReapsIdleRegardlessOfMaxIdleConnections pins the bug this
// package shipped with: MaxConnIdleTime used to be set only when
// SetMaxIdleConnections was above zero, so a deployment that never tuned the
// idle-connection count silently kept the pgx default of 30 minutes and held a
// whole max_connections budget open long after a load run finished.
func TestBuildPoolConfigReapsIdleRegardlessOfMaxIdleConnections(t *testing.T) {
	tests := []struct {
		name         string
		maxIdleConns int
		maxOpenConns int
		wantMaxConns int32
	}{
		{
			name:         "max idle connections unset",
			maxIdleConns: 0,
			maxOpenConns: 40,
			wantMaxConns: 40,
		},
		{
			name:         "max idle connections negative",
			maxIdleConns: -1,
			maxOpenConns: 40,
			wantMaxConns: 40,
		},
		{
			name:         "max idle connections set",
			maxIdleConns: 10,
			maxOpenConns: 40,
			wantMaxConns: 40,
		},
		{
			name:         "both pool knobs unset",
			maxIdleConns: 0,
			maxOpenConns: 0,
			wantMaxConns: config.DefaultMaxOpenConnections,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dbCfg := poolTestDBConfig()
			dbCfg.SetMaxIdleConnections = tc.maxIdleConns
			dbCfg.SetMaxOpenConnections = tc.maxOpenConns

			pgxCfg, sink, err := buildPoolConfig(dbCfg, &captureLogger{})
			require.NoError(t, err)
			require.NotNil(t, sink)

			require.Equal(t, maxConnIdleTime, pgxCfg.MaxConnIdleTime,
				"idle connections must be reaped whether or not max_idle_conn is set")
			require.Equal(t, tc.wantMaxConns, pgxCfg.MaxConns)
		})
	}
}

// TestBuildPoolConfigRejectsUnusableDSN keeps the error path returning a nil
// config rather than a half-built one a caller could still hand to pgx.
func TestBuildPoolConfigRejectsUnusableDSN(t *testing.T) {
	dbCfg := poolTestDBConfig()
	dbCfg.DSN = "://not-a-dsn"

	pgxCfg, sink, err := buildPoolConfig(dbCfg, &captureLogger{})
	require.Error(t, err)
	require.Nil(t, pgxCfg)
	require.Nil(t, sink)
}
