package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	"github.com/uptrace/opentelemetry-go-extra/otelsqlx"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"

	"github.com/frain-dev/convoy/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/newrelic/go-agent/v3/integrations/nrpq"
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
	dbConfig := cfg.Database
	db, err := otelsqlx.Connect("postgres", dbConfig.BuildDsn(),
		otelsql.WithDBName("postgres"),
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL))

	log.Info(dbConfig)

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
