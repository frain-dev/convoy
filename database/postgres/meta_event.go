package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

var (
	ErrMetaEventNotCreated = errors.New("metaevent could not be created")
)

const (
	createMetaEvent = `
	INSERT INTO convoy.meta_events (id, event_type, project_id, data, status, retry_count, max_retry_count)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	fetchMetaEventById = `
	SELECT * from convoy.meta_events WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`
)

type metaEventRepo struct {
	db *sqlx.DB
}

func NewMetaEventRepo(db database.Database) datastore.MetaEventRepository {
	return &metaEventRepo{db: db.GetDB()}
}

func (m *metaEventRepo) CreateMetaEvent(ctx context.Context, metaEvent *datastore.MetaEvent) error {
	r, err := m.db.ExecContext(ctx, createMetaEvent, metaEvent.UID, metaEvent.EventType, metaEvent.ProjectID,
		metaEvent.Data, metaEvent.Status, metaEvent.RetryCount, metaEvent.MaxRetryCount,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrMetaEventNotCreated
	}

	return nil
}

func (m *metaEventRepo) FindMetaEventByID(ctx context.Context, projectID string, id string) (*datastore.MetaEvent, error) {
	metaEvent := &datastore.MetaEvent{}
	err := m.db.QueryRowxContext(ctx, fetchMetaEventById, id, projectID).StructScan(metaEvent)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrMetaEventNotFound
		}

		return nil, err
	}

	return metaEvent, nil
}

func (m *metaEventRepo) LoadMetaEventsPaged(ctx context.Context, projectID string, f *datastore.Filter) ([]datastore.MetaEvent, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}
