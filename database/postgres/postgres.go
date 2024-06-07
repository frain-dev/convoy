package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"io"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/jmoiron/sqlx"
)

const pkgName = "postgres"

type DbCtxKey string

const TransactionCtx DbCtxKey = "transaction"

// ErrPendingMigrationsFound is used to indicate there exist pending migrations yet to be run
// if the user proceeds without running migrations it can lead to data integrity issues.
var ErrPendingMigrationsFound = errors.New("migrate: Pending migrations exist, please run convoy migrate first")

type Postgres struct {
	dbx  *sqlx.DB
	hook *hooks.Hook
	pool *pgxpool.Pool
}

func NewDB(cfg config.Configuration) (*Postgres, error) {
	dbConfig := cfg.Database

	pgxCfg, err := pgxpool.ParseConfig(dbConfig.BuildDsn())
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	pgxCfg.MaxConnLifetime = time.Second * time.Duration(dbConfig.SetConnMaxLifetime)
	pgxCfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())

	pool, err := pgxpool.NewWithConfig(context.Background(), pgxCfg)
	if err != nil {
		defer pool.Close()
		return nil, fmt.Errorf("[%s]: failed to open database - %v", pkgName, err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	db := sqlx.NewDb(sqlDB, "pgx")

	return &Postgres{dbx: db, pool: pool}, nil
}

func (p *Postgres) GetDB() *sqlx.DB {
	return p.dbx
}

func (p *Postgres) Close() error {
	p.pool.Close()
	return p.dbx.Close()
}

func (p *Postgres) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return p.dbx.BeginTxx(ctx, nil)
}

func (p *Postgres) Rollback(tx *sqlx.Tx, err error) {
	if err != nil {
		rbErr := tx.Rollback()
		log.WithError(rbErr).Error("failed to roll back transaction in ProcessBroadcastEventCreation")
	}

	cmErr := tx.Commit()
	if cmErr != nil && !errors.Is(cmErr, sql.ErrTxDone) {
		log.WithError(cmErr).Error("failed to commit tx in ProcessBroadcastEventCreation, rolling back transaction")
		rbErr := tx.Rollback()
		log.WithError(rbErr).Error("failed to roll back transaction in ProcessBroadcastEventCreation")
	}
}

func (p *Postgres) GetHook() *hooks.Hook {
	if p.hook != nil {
		return p.hook
	}

	hook, err := hooks.Get()
	if err != nil {
		log.Fatal(err)
	}

	p.hook = hook
	return p.hook
}

func GetTx(ctx context.Context, db *sqlx.DB) (*sqlx.Tx, bool, error) {
	isWrapped := false

	wrappedTx, ok := ctx.Value(TransactionCtx).(*sqlx.Tx)
	if ok && wrappedTx != nil {
		isWrapped = true
		return wrappedTx, isWrapped, nil
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, isWrapped, err
	}

	return tx, isWrapped, nil
}

func rollbackTx(tx *sqlx.Tx) {
	err := tx.Rollback()
	if err != nil && !errors.Is(err, sql.ErrTxDone) {
		log.WithError(err).Error("failed to rollback tx")
	}
}

func closeWithError(closer io.Closer) {
	err := closer.Close()
	if err != nil {
		fmt.Printf("%v, an error occurred while closing the client", err)
	}
}
