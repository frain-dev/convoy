package postgres

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const pkgName = "postgres"

type Postgres struct {
	dbx *sqlx.DB
}

func NewDB() (*Postgres, error) {
	db, err := sqlx.Connect("postgres", "postgres://admin:password@localhost/convoy?sslmode=disable")
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
