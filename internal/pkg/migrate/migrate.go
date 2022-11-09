package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	DefaultOptions = &Options{
		DatabaseName:              "convoy",
		CollectionName:            "data_migrations",
		ValidateUnknownMigrations: true,
		UseTransaction:            true,
	}

	// ErrRollbackImpossible is returned when trying to rollback a migration
	// that has no rollback function.
	ErrRollbackImpossible = errors.New("migrate: It's impossible to rollback this migration")

	// ErrNoMigrationDefined is returned when no migration is defined.
	ErrNoMigrationDefined = errors.New("migrate: No migration defined")

	// ErrMissingID is returned when the ID of the migration is equal to ""
	ErrMissingID = errors.New("migrate: Missing ID in migration")

	// ErrNoRunMigration is returned when any run migration was found while
	// running RollbackLast
	ErrNoRunMigration = errors.New("migrate: Could not find last run migration")

	// ErrUnknownPastMigration is returned if a migration exists in the DB that doesn't exist in the code
	ErrUnknownPastMigration = errors.New("migrate: Found migration in DB that does not exist in code")

	// ErrMigrationIDDoesNotExist is returned when migrating or rolling back to a migration ID that
	// does not exist in the list of migrations
	ErrMigrationIDDoesNotExist = errors.New("migrate: Tried to migrate to an ID that doesn't exist")

	// ErrPendingMigrationsFound is used to indicate there exist pending migrations yet to be run
	// if the user proceeds without running migrations it can lead to data integrity issues.
	ErrPendingMigrationsFound = errors.New("migrate: Pending migrations exist, please run convoy migrate first")
)

// DuplicatedIDError is returned when more than one migration have the same ID
type DuplicatedIDError struct {
	ID string
}

func (e *DuplicatedIDError) Error() string {
	return fmt.Sprintf(`gormigrate: Duplicated migration ID: "%s"`, e.ID)
}

type Options struct {
	// DatabaseName is the name of the database to connect.
	DatabaseName string

	// CollectionName is the migration table.
	CollectionName string

	// ValidateUnknownMigrations will cause migrate to fail if there's unknown migration
	// IDs in the database
	ValidateUnknownMigrations bool

	// UseTransaction makes Gormigrate execute migrations inside a single transaction.
	UseTransaction bool
}

type MigrationDoc struct {
	ID string `json:"id" bson:"id"`
}

type InitSchemaFunc func(context.Context, *mongo.Database) (bool, error)

type MigrateFunc func(*mongo.Database) error

type RollbackFunc func(*mongo.Database) error

type Migration struct {
	// ID is the migration identifier. Usually a timestamp like "201601021504".
	ID string
	// Migrate is a function that will br executed while running this migration.
	Migrate MigrateFunc
	// Rollback will be executed on rollback. Can be nil.
	Rollback RollbackFunc
}

type Migrator struct {
	client     *mongo.Client
	session    mongo.Session
	db         *mongo.Database
	opts       *Options
	migrations []*Migration
	initSchema InitSchemaFunc
}

func NewMigrator(c *mongo.Client, opts *Options, migrations []*Migration, i InitSchemaFunc) *Migrator {
	if opts.DatabaseName == "" {
		opts.DatabaseName = DefaultOptions.DatabaseName
	}

	if opts.CollectionName == "" {
		opts.CollectionName = DefaultOptions.CollectionName
	}

	m := &Migrator{
		client:     c,
		db:         c.Database(opts.DatabaseName, nil),
		opts:       opts,
		migrations: migrations,
	}

	if i == nil {
		i = m.defaultinitSchema
	}

	m.initSchema = i

	return m
}

// Migrate executes all migrations that did not run yet.
func (m *Migrator) Migrate(ctx context.Context) error {
	if !m.hasMigrations() {
		return ErrNoMigrationDefined
	}

	var targetMigrationID string
	if len(m.migrations) > 0 {
		targetMigrationID = m.migrations[len(m.migrations)-1].ID
	}

	return m.migrate(ctx, targetMigrationID)
}

// MigrateTo executes all migrations that did not run yet up to the migration that matches `migrationID`.
func (m *Migrator) MigrateTo(ctx context.Context, migrationID string) error {
	if err := m.checkIDExist(migrationID); err != nil {
		return err
	}

	return m.migrate(ctx, migrationID)
}

// RollbackLast undo the last migration
func (m *Migrator) RollbackLast(ctx context.Context) error {
	if !m.hasMigrations() {
		return ErrNoMigrationDefined
	}

	m.begin()
	defer m.rollback(ctx)

	lastRunMigration, err := m.getLastRunMigration()
	if err != nil {
		return err
	}

	if err := m.rollbackMigration(ctx, lastRunMigration); err != nil {
		return err
	}

	return m.commit()
}

// RollbackTo undoes migrations up to the given migration that matches the `migrationID`.
// Migration with the matching `migrationID` is not rolled back.
func (m *Migrator) RollbackTo(ctx context.Context, migrationID string) error {
	if !m.hasMigrations() {
		return ErrNoMigrationDefined
	}

	if err := m.checkIDExist(migrationID); err != nil {
		return err
	}

	m.begin()
	defer m.rollback(ctx)

	for i := len(m.migrations) - 1; i >= 0; i-- {
		migration := m.migrations[i]
		if migration.ID == migrationID {
			break
		}

		migrationRan, err := m.migrationRan(migration)
		if err != nil {
			return err
		}

		if migrationRan {
			if err := m.rollbackMigration(ctx, migration); err != nil {
				return err
			}
		}
	}

	return m.commit()
}

func (m *Migrator) CheckPendingMigrations(ctx context.Context) (bool, error) {
	store := datastore.New(m.db)
	ctx = context.WithValue(ctx, datastore.CollectionCtx, m.opts.CollectionName)

	filter := bson.M{
		"id": bson.M{
			"$ne": "schema_init",
		},
	}

	dbMigrations, err := store.Count(ctx, filter)
	if err != nil {
		return false, err
	}

	if len(m.migrations) > int(dbMigrations) {
		return true, nil
	}

	return false, nil
}

func (m *Migrator) migrate(ctx context.Context, migrationID string) error {
	if !m.hasMigrations() {
		return ErrNoMigrationDefined
	}

	if err := m.checkDuplicatedID(); err != nil {
		return err
	}

	m.begin()
	defer m.rollback(ctx)

	if m.opts.ValidateUnknownMigrations {
		unknownMigrations, err := m.unknownMigrationsHaveHappened()
		if err != nil {
			return err
		}

		if unknownMigrations {
			return ErrUnknownPastMigration
		}
	}

	initializedSchema, err := m.initSchema(ctx, m.db)
	if err != nil {
		return err
	}

	if initializedSchema {
		return m.commit()
	}

	for _, migration := range m.migrations {
		if err := m.runMigration(ctx, migration); err != nil {
			return err
		}
		if migrationID != "" && migration.ID == migrationID {
			break
		}
	}

	return m.commit()
}

func (m *Migrator) unknownMigrationsHaveHappened() (bool, error) {
	store := datastore.New(m.db)
	ctx := context.WithValue(context.Background(), datastore.CollectionCtx, m.opts.CollectionName)

	var appliedMigrations []*MigrationDoc
	_, err := store.FindMany(ctx, nil, nil, nil, 0, 0, &appliedMigrations)
	if err != nil {
		return false, err
	}

	validIDSet := make(map[string]bool, len(m.migrations)+1)
	for _, migration := range m.migrations {
		validIDSet[migration.ID] = true
	}

	for _, migration := range appliedMigrations {
		if _, ok := validIDSet[migration.ID]; !ok {
			return true, nil
		}
	}

	return false, nil
}

func (m *Migrator) begin() {
	if m.opts.UseTransaction {
		session, err := m.client.StartSession()
		if err != nil {
			log.WithError(err).Fatalf("Could not start mongodb session - %v", err)
		}
		m.session = session
	}
}

func (m *Migrator) rollback(ctx context.Context) {
	if m.opts.UseTransaction {
		m.session.EndSession(ctx)
	}
}

func (m *Migrator) commit() error {
	if m.opts.UseTransaction {
		return m.session.CommitTransaction(context.Background())
	}
	return nil
}

func (m *Migrator) migrationRan(migration *Migration) (bool, error) {
	var count int64

	store := datastore.New(m.db)
	ctx := context.WithValue(context.Background(), datastore.CollectionCtx, m.opts.CollectionName)

	filter := map[string]interface{}{
		"id": migration.ID,
	}

	count, err := store.CountWithDeleted(ctx, filter)

	return count > 0, err
}

func (m *Migrator) insertMigration(ctx context.Context, id string) error {
	store := datastore.New(m.db)
	ctx = context.WithValue(ctx, datastore.CollectionCtx, m.opts.CollectionName)

	var result MigrationDoc
	payload := &MigrationDoc{ID: id}

	if m.session != nil {
		ctx = mongo.NewSessionContext(ctx, m.session)
	}

	err := store.Save(ctx, payload, &result)
	if err != nil {
		return err
	}

	return nil
}

func (m *Migrator) runMigration(ctx context.Context, migration *Migration) error {
	if len(migration.ID) == 0 {
		return ErrMissingID
	}

	migrationRan, err := m.migrationRan(migration)
	if err != nil {
		return err
	}

	if !migrationRan {
		if err := migration.Migrate(m.db); err != nil {
			return err
		}

		if err := m.insertMigration(ctx, migration.ID); err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) checkDuplicatedID() error {
	lookup := make(map[string]bool, len(m.migrations))
	for _, m := range m.migrations {
		if _, ok := lookup[m.ID]; ok {
			return &DuplicatedIDError{ID: m.ID}
		}
		lookup[m.ID] = true
	}
	return nil
}

// There are migrations to apply if the list of migrations is not empty.
func (m *Migrator) hasMigrations() bool {
	return len(m.migrations) > 0
}

func (m *Migrator) checkIDExist(migrationID string) error {
	if migrationID == "schema_init" {
		return nil
	}

	for _, migration := range m.migrations {
		if migration.ID == migrationID {
			return nil
		}
	}

	return ErrMigrationIDDoesNotExist
}

func (m *Migrator) rollbackMigration(ctx context.Context, migration *Migration) error {
	if migration.Rollback == nil {
		return ErrRollbackImpossible
	}

	if err := migration.Rollback(m.db); err != nil {
		return err
	}

	store := datastore.New(m.db)
	ctx = context.WithValue(ctx, datastore.CollectionCtx, m.opts.CollectionName)

	filter := map[string]interface{}{
		"id": migration.ID,
	}

	if m.session != nil {
		ctx = mongo.NewSessionContext(ctx, m.session)
	}

	return store.DeleteOne(ctx, filter, true)
}

func (m *Migrator) getLastRunMigration() (*Migration, error) {
	for i := len(m.migrations) - 1; i >= 0; i-- {
		migration := m.migrations[i]

		migrationRan, err := m.migrationRan(migration)
		if err != nil {
			return nil, err
		}

		if migrationRan {
			return migration, nil
		}

	}

	return nil, ErrNoRunMigration
}

func (m *Migrator) defaultinitSchema(ctx context.Context, db *mongo.Database) (bool, error) {
	// save the last schema if nothing dey.
	filter := map[string]interface{}{}

	store := datastore.New(m.db)
	ctx = context.WithValue(ctx, datastore.CollectionCtx, m.opts.CollectionName)

	count, err := store.Count(ctx, filter)
	if err != nil {
		return false, err
	}

	if count == 0 {
		err := m.insertMigration(ctx, m.migrations[len(m.migrations)-1].ID)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}
