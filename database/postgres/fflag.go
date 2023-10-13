package postgres

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/database"

	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

const (
	saveFFlags = `
	INSERT INTO convoy.feature_flags (id,feature_key,type)
	VALUES (:id, :feature_key, :type)
	`

	clearFFlagTable = `DELETE * FROM convoy.feature_flags`
)

type fflagRepo struct {
	db *sqlx.DB
}

func NewFFlagRepo(db database.Database) datastore.FFlagRepository {
	return &fflagRepo{db: db.GetDB()}
}

func (e *fflagRepo) ClearFlagTable(ctx context.Context) error {
	result, err := e.db.ExecContext(ctx, clearFFlagTable)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if n < 1 {
		return errors.New("failed to clear feature flag table")
	}

	return nil
}

func (e *fflagRepo) SaveFlags(ctx context.Context, flags []datastore.Flag) error {
	_, err := e.db.NamedExecContext(ctx, saveFFlags, flags)
	return err
}
