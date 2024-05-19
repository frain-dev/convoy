package migrator

import (
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database"
	"github.com/jmoiron/sqlx"
	migrate "github.com/rubenv/sql-migrate"
)

var (
	tableSchema = "convoy"
)

type MigrationRecord struct {
	Name      string
	AppliedAt time.Time
}

type Migrator struct {
	dbx *sqlx.DB
	src migrate.MigrationSource
}

func New(d database.Database) *Migrator {
	migrations := &migrate.EmbedFileSystemMigrationSource{
		FileSystem: convoy.MigrationFiles,
		Root:       "sql",
	}

	migrate.SetSchema(tableSchema)
	return &Migrator{dbx: d.GetDB(), src: migrations}
}

func (m *Migrator) Up() error {
	_, err := migrate.Exec(m.dbx.DB, "postgres", m.src, migrate.Up)
	if err != nil {
		return err
	}
	return nil
}

func (m *Migrator) Down(max int) error {
	_, err := migrate.ExecMax(m.dbx.DB, "postgres", m.src, migrate.Down, max)
	if err != nil {
		return err
	}
	return nil
}

func (m *Migrator) List() ([]MigrationRecord, error) {
	records, err := migrate.GetMigrationRecords(m.dbx.DB, "postgres")
	if err != nil {
		return nil, err
	}

	var migrationRecords []MigrationRecord
	for _, record := range records {
		migrationRecord := MigrationRecord{
			Name:      record.Id,
			AppliedAt: record.AppliedAt,
		}
		migrationRecords = append(migrationRecords, migrationRecord)
	}

	return migrationRecords, nil
}
