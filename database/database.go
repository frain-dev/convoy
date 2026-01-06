package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy/database/hooks"
)

type Database interface {
	GetConn() *pgxpool.Pool
	GetDB() *sqlx.DB
	GetReadDB() *sqlx.DB
	BeginTx(context.Context) (*sqlx.Tx, error)
	GetHook() *hooks.Hook
	Rollback(tx *sqlx.Tx, err error)
	Close() error
}
