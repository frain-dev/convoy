package database

import (
	"github.com/frain-dev/convoy/database/sqlite3"
	"github.com/jmoiron/sqlx"
)

type Database interface {
	GetDB() *sqlx.DB
}

func New() (Database, error) {
	db, err := sqlite3.NewDB()
	if err != nil {
		return nil, err
	}

	return db, nil
}
