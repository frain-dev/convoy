package sqlite3

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const pkgName = "sqlite3"

type Sqlite struct {
	dbx *sqlx.DB
}

func NewDB() (*Sqlite, error) {
	db, err := sqlx.Connect("sqlite3", "convoy.db")
	if err != nil {
		return nil, fmt.Errorf("[%s]: failed to open database - %v", pkgName, err)
	}
	return &Sqlite{dbx: db}, nil
}

func (s *Sqlite) GetDB() *sqlx.DB {
	return s.dbx
}
