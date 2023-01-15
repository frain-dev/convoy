package migrator

import (
	"github.com/frain-dev/convoy/database"
	"github.com/jmoiron/sqlx"
	migrate "github.com/rubenv/sql-migrate"
)

type Migrator struct {
	dbx *sqlx.DB
	src migrate.MigrationSource
}

func New(d database.Database) *Migrator {
	migrations := &migrate.FileMigrationSource{
		Dir: "sql",
	}
	return &Migrator{dbx: d.GetDB(), src: migrations}
}

func (m *Migrator) Up() error {
	_, err := migrate.ExecMax(m.dbx.DB, "postgres", m.src, migrate.Up, 1)
	if err != nil {
		return err
	}
	return nil
}

func (m *Migrator) Down() error {
	_, err := migrate.ExecMax(m.dbx.DB, "postgres", m.src, migrate.Down, 1)
	if err != nil {
		return err
	}
	return nil
}
