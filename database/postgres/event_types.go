package postgres

import (
	"context"
	"database/sql"
	"errors"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

var (
	ErrEventTypeNotFound   = errors.New("event type not found")
	ErrEventTypeNotCreated = errors.New("event type could not be created")
	ErrEventTypeNotUpdated = errors.New("event type could not be updated")
)

const (
	createEventType = `
	INSERT INTO convoy.event_types (id, name, description, category, project_id, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, now(), now());
	`

	updateEventType = `
	UPDATE convoy.event_types SET
	description = $3,
	category = $4,
	updated_at = NOW()
	WHERE id = $1 and project_id = $2;
	`

	deprecateEventType = `
	UPDATE convoy.event_types SET
	deprecated_at = NOW() 
	WHERE id = $1 and project_id = $2 
	returning *;
	`

	fetchEventTypeById = `
	SELECT * FROM convoy.event_types
	WHERE id = $1 and project_id = $2;
	`

	fetchAllEventTypes = `
	SELECT * FROM convoy.event_types where project_id = $1;
	`
)

type eventTypesRepo struct {
	db    *sqlx.DB
	cache cache.Cache
}

func NewEventTypesRepo(db database.Database) datastore.EventTypesRepository {
	return &eventTypesRepo{db: db.GetDB()}
}

func (e *eventTypesRepo) CreateEventType(ctx context.Context, eventType *datastore.ProjectEventType) error {
	r, err := e.db.ExecContext(ctx, createEventType,
		eventType.ID,
		eventType.Name,
		eventType.Description,
		eventType.Category,
		eventType.ProjectId,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEventTypeNotCreated
	}

	return nil
}

func (e *eventTypesRepo) UpdateEventType(ctx context.Context, eventType *datastore.ProjectEventType) error {
	r, err := e.db.ExecContext(ctx, updateEventType,
		eventType.ID,
		eventType.ProjectId,
		eventType.Description,
		eventType.Category,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEventTypeNotUpdated
	}

	return nil
}

func (e *eventTypesRepo) DeprecateEventType(ctx context.Context, id, projectId string) (*datastore.ProjectEventType, error) {
	eventType := &datastore.ProjectEventType{}
	err := e.db.QueryRowxContext(ctx, deprecateEventType, id, projectId).StructScan(eventType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEventTypeNotFound
		}
		return nil, err
	}

	return eventType, nil
}

func (e *eventTypesRepo) FetchEventTypeById(ctx context.Context, id, projectId string) (*datastore.ProjectEventType, error) {
	eventType := &datastore.ProjectEventType{}
	err := e.db.QueryRowxContext(ctx, fetchEventTypeById, id, projectId).StructScan(eventType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEventTypeNotFound
		}
		return nil, err
	}

	return eventType, nil
}

func (e *eventTypesRepo) FetchAllEventTypes(ctx context.Context, projectId string) ([]datastore.ProjectEventType, error) {
	var eventTypes []datastore.ProjectEventType
	rows, err := e.db.QueryxContext(ctx, fetchAllEventTypes, projectId)
	if err != nil {
		return nil, err
	}
	defer closeWithError(rows)

	for rows.Next() {
		var eventType datastore.ProjectEventType

		err = rows.StructScan(&eventType)
		if err != nil {
			return nil, err
		}

		eventTypes = append(eventTypes, eventType)
	}

	return eventTypes, nil
}
