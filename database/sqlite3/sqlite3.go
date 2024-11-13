package sqlite3

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/pkg/log"
	"io"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const pkgName = "sqlite3"

type Sqlite struct {
	dbx    *sqlx.DB
	hook   *hooks.Hook
	logger *log.Logger
}

func (s *Sqlite) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return s.dbx.BeginTxx(ctx, nil)
}

func (s *Sqlite) GetHook() *hooks.Hook {
	if s.hook != nil {
		return s.hook
	}

	hook, err := hooks.Get()
	if err != nil {
		log.Fatal(err)
	}

	s.hook = hook
	return s.hook
}

func (s *Sqlite) Rollback(tx *sqlx.Tx, err error) {
	if err != nil {
		rbErr := tx.Rollback()
		log.WithError(rbErr).Error("failed to roll back transaction in ProcessBroadcastEventCreation")
	}

	cmErr := tx.Commit()
	if cmErr != nil && !errors.Is(cmErr, sql.ErrTxDone) {
		log.WithError(cmErr).Error("failed to commit tx in ProcessBroadcastEventCreation, rolling back transaction")
		rbErr := tx.Rollback()
		log.WithError(rbErr).Error("failed to roll back transaction in ProcessBroadcastEventCreation")
	}
}

func (s *Sqlite) Close() error {
	return s.dbx.Close()
}

func NewDB(dbName string, logger *log.Logger) (*Sqlite, error) {
	db, err := sqlx.Connect("sqlite3", dbName)
	if err != nil {
		return nil, fmt.Errorf("[%s]: failed to open database - %v", pkgName, err)
	}
	return &Sqlite{dbx: db, logger: logger}, nil
}

func (s *Sqlite) GetDB() *sqlx.DB {
	return s.dbx
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
