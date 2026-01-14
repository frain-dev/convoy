package migrator

import (
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"

	migrate "github.com/rubenv/sql-migrate"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
)

var (
	tableSchema = "convoy"
)

type Migrator struct {
	dbx    *sqlx.DB
	src    migrate.MigrationSource
	logger log.StdLogger
}

func New(d database.Database) *Migrator {
	defaultLogger := log.NewLogger(os.Stdout)
	return NewWithLogger(d, defaultLogger)
}

func NewWithLogger(d database.Database, logger log.StdLogger) *Migrator {
	migrations := &migrate.EmbedFileSystemMigrationSource{
		FileSystem: convoy.MigrationFiles,
		Root:       "sql",
	}

	migrate.SetSchema(tableSchema)

	if logger == nil {
		logger = log.NewLogger(os.Stdout)
	}

	return &Migrator{dbx: d.GetDB(), src: migrations, logger: logger}
}

func (m *Migrator) Up() error {
	applied, err := migrate.Exec(m.dbx.DB, "postgres", m.src, migrate.Up)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	if applied > 0 {
		m.logger.Infof("Applied %d migration(s)", applied)
	} else {
		m.logger.Info("No pending migrations")
	}
	return nil
}

func (m *Migrator) Down(maxMigrations int) error {
	rolledBack, err := migrate.ExecMax(m.dbx.DB, "postgres", m.src, migrate.Down, maxMigrations)
	if err != nil {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}
	if rolledBack > 0 {
		m.logger.Infof("Rolled back %d migration(s)", rolledBack)
	} else {
		m.logger.Info("No migrations to rollback")
	}
	return nil
}
