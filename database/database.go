package database

import (
	"context"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/jmoiron/sqlx"
)

type Database interface {
	GetDB() *sqlx.DB
	BeginTx(context.Context) (*sqlx.Tx, error)
	GetHook() *hooks.Hook
	Rollback(tx *sqlx.Tx, err error)
	Close() error
}
