package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx"
	_ "github.com/jackc/pgx/stdlib"
	"io"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

const pkgName = "postgres"

// ErrPendingMigrationsFound is used to indicate there exist pending migrations yet to be run
// if the user proceeds without running migrations it can lead to data integrity issues.
var ErrPendingMigrationsFound = errors.New("migrate: Pending migrations exist, please run convoy migrate first")

type Postgres struct {
	dbx  *sqlx.DB
	hook *hooks.Hook
}

func NewDB(cfg config.Configuration) (*Postgres, error) {
	connConfig, err := pgx.ParseConnectionString(cfg.Database.BuildDsn())
	if err != nil {
		return nil, fmt.Errorf("[%s]: failed to parse db dsn - %v", pkgName, err)
	}

	connPool, err := pgx.NewConnPool(pgx.ConnPoolConfig{
		ConnConfig:     connConfig,
		MaxConnections: cfg.Database.SetMaxOpenConnections,
		AcquireTimeout: time.Second * time.Duration(cfg.Database.SetConnMaxLifetime),
	})
	if err != nil {
		connPool.Close()
		return nil, fmt.Errorf("[%s]: failed to configure connection pool - %v", pkgName, err)
	}

	pgxdb := stdlib.OpenDBFromPool(connPool)

	db := sqlx.NewDb(pgxdb, "pgx")
	if err != nil {
		return nil, fmt.Errorf("[%s]: failed to open database - %v", pkgName, err)
	}

	db.SetMaxIdleConns(cfg.Database.SetMaxIdleConnections)
	db.SetMaxOpenConns(cfg.Database.SetMaxOpenConnections)
	db.SetConnMaxLifetime(time.Second * time.Duration(cfg.Database.SetConnMaxLifetime))

	return &Postgres{dbx: db}, nil
}

func (p *Postgres) GetDB() *sqlx.DB {
	return p.dbx
}

func (p *Postgres) Close() error {
	return p.dbx.Close()
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
