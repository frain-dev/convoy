package broker

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy/config"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/worker/task"
)

// jobLockMaxConns caps the dedicated advisory-lock pool. Callers are a fixed
// set of instance cron mutexes (not per-tenant), so this stays small and
// cannot starve the shared API/worker pool (default max 100).
const jobLockMaxConns = 16

// newPostgresJobLockerFromDB builds the postgres JobLocker. Production wires a
// dedicated lock pool via newPostgresJobLockerFromConfig; tests may pass a mock DB.
func newPostgresJobLockerFromDB(db *sqlx.DB, logger log.Logger) task.JobLocker {
	return newPostgresJobLockerWithLimit(db, logger, jobLockMaxConns)
}

func newPostgresJobLockerWithLimit(db *sqlx.DB, logger log.Logger, maxConns int) *postgresJobLocker {
	if maxConns <= 0 {
		maxConns = jobLockMaxConns
	}
	// Enforce the bound on whatever DB we were given (dedicated pool or test mock).
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns)
	return &postgresJobLocker{
		db:     db,
		logger: logger,
		slots:  make(chan struct{}, maxConns),
	}
}

// openJobLockDB opens a dedicated pool used only for session advisory locks.
var openJobLockDB = defaultOpenJobLockDB

func defaultOpenJobLockDB(cfg config.DatabaseConfiguration) (*sqlx.DB, error) {
	dsn := cfg.BuildDsn()
	if dsn == "" {
		return nil, fmt.Errorf("postgres job locker requires a database dsn")
	}
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse job lock database config: %w", err)
	}
	poolCfg.MaxConns = int32(jobLockMaxConns)
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("open job lock database pool: %w", err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	db := sqlx.NewDb(sqlDB, "pgx")
	db.SetMaxOpenConns(jobLockMaxConns)
	db.SetMaxIdleConns(jobLockMaxConns)
	db.SetConnMaxLifetime(time.Second * time.Duration(cfg.SetConnMaxLifetime))
	return db, nil
}
