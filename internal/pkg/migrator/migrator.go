package migrator

import (
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database"
	"github.com/jmoiron/sqlx"
	migrate "github.com/rubenv/sql-migrate"
)

var (
	tableSchema = "convoy"
)

type Migrator struct {
	dbx     *sqlx.DB
	dialect string
	src     migrate.MigrationSource
}

func New(d database.Database, dialect string) *Migrator {
	migrations := &migrate.EmbedFileSystemMigrationSource{
		FileSystem: convoy.SQLiteMigrationFiles,
		Root:       "sql/sqlite3",
	}

	if dialect == "postgres" {
		migrations.FileSystem = convoy.PostgresMigrationFiles
		migrations.Root = "sql/postgres"
		migrate.SetSchema(tableSchema)
	}

	return &Migrator{dbx: d.GetDB(), src: migrations, dialect: dialect}
}

func (m *Migrator) Up() error {
	_, err := migrate.Exec(m.dbx.DB, m.dialect, m.src, migrate.Up)
	if err != nil {
		return err
	}
	return nil
}

func (m *Migrator) Down(maxDown int) error {
	_, err := migrate.ExecMax(m.dbx.DB, m.dialect, m.src, migrate.Down, maxDown)
	if err != nil {
		return err
	}
	return nil
}
