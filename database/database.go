package database

import (
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/jmoiron/sqlx"
)

type Database interface {
	GetDB() *sqlx.DB
	GetHook() *hooks.Hook
	Close() error
}
