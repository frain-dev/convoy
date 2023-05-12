package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const pkgName = "postgres"

// ErrPendingMigrationsFound is used to indicate there exist pending migrations yet to be run
// if the user proceeds without running migrations it can lead to data integrity issues.
var ErrPendingMigrationsFound = errors.New("migrate: Pending migrations exist, please run convoy migrate first")

type Postgres struct {
	dbx *sqlx.DB
}

func NewDB(cfg config.Configuration) (*Postgres, error) {
	dbConfig := cfg.Database
	println(dbConfig.BuildDsn())
	db, err := sqlx.Connect("postgres", dbConfig.BuildDsn())
	if err != nil {
		return nil, fmt.Errorf("[%s]: failed to open database - %v", pkgName, err)
	}

	db.SetMaxIdleConns(dbConfig.SetMaxIdleConnections)
	db.SetMaxOpenConns(dbConfig.SetMaxOpenConnections)
	db.SetConnMaxLifetime(time.Second * time.Duration(dbConfig.SetConnMaxLifetime))

	return &Postgres{dbx: db}, nil
}

func (p *Postgres) GetDB() *sqlx.DB {
	return p.dbx
}

func (p *Postgres) Close() error {
	return p.dbx.Close()
}

func rollbackTx(tx *sqlx.Tx) {
	err := tx.Rollback()
	if err != nil && !errors.Is(err, sql.ErrTxDone) {
		log.WithError(err).Error("failed to rollback tx")
	}
}
