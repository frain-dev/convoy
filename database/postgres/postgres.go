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

type Postgres struct {
	dbx *sqlx.DB
}

func NewDB(cfg config.Configuration) (*Postgres, error) {
	db, err := sqlx.Connect("postgres", cfg.Database.Dsn)
	if err != nil {
		return nil, fmt.Errorf("[%s]: failed to open database - %v", pkgName, err)
	}

	db.SetMaxIdleConns(10)                    // The default is defaultMaxIdleConns (= 2)
	db.SetMaxOpenConns(1000)                  // The default is 0 (unlimited)
	db.SetConnMaxLifetime(3600 * time.Second) // The default is 0 (connections reused forever)

	return &Postgres{dbx: db}, nil
}

func (p *Postgres) GetDB() *sqlx.DB {
	return p.dbx
}

func (p *Postgres) Close() error {
	return p.dbx.Close()
}

// getPrevPage returns calculated value for the prev page
func getPrevPage(page int) int {
	if page == 0 {
		return 1
	}

	prev := 0
	if page-1 <= 0 {
		prev = page
	} else {
		prev = page - 1
	}

	return prev
}

func rollbackTx(tx *sqlx.Tx) {
	err := tx.Rollback()
	if err != nil && !errors.Is(err, sql.ErrTxDone) {
		log.WithError(err).Error("failed to rollback tx")
	}
}
