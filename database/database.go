package database

import (
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/jmoiron/sqlx"
)

type Database interface {
	GetDB() *sqlx.DB
}

func New() (Database, error) {
	db, err := postgres.NewDB()
	if err != nil {
		return nil, err
	}

	return db, nil
}
