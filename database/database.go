package database

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy/database/hooks"
)

type Database interface {
	GetDB() *sqlx.DB
	GetReadDB() *sqlx.DB
	BeginTx(context.Context) (*sqlx.Tx, error)
	GetHook() *hooks.Hook
	Rollback(tx *sqlx.Tx, err error)
	Close() error
}
