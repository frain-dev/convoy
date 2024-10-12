package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

type eventCatalogueRepo struct {
	db    *sqlx.DB
	cache cache.Cache
}

const (
	createEventCatalogue = `
	INSERT INTO convoy.event_catalogues (id,project_id,
	                           type,events,open_api_spec,created_at,updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	fetchCatalogue = `SELECT * FROM convoy.event_catalogues WHERE deleted_at IS NULL AND project_id = $1`

	deleteCatalogue = `UPDATE convoy.event_catalogues SET deleted_at = NOW() WHERE id = $1 AND project_id = $2`

	updateCatalogue = `UPDATE convoy.event_catalogues SET events = $1, open_api_spec = $2 WHERE id = $3 AND project_id = $4`
)

var (
	ErrEventCatalogueNotCreated = errors.New("event catalogue could not be created")
	ErrEventCatalogueNotUpdated = errors.New("event catalogue could not be updated")
	ErrEventCatalogueExists     = errors.New("this project already has a catalogue")
)

func (e *eventCatalogueRepo) CreateEventCatalogue(ctx context.Context, catalogue *datastore.EventCatalogue) error {
	result, err := e.db.ExecContext(ctx, createEventCatalogue,
		catalogue.UID, catalogue.ProjectID,
		catalogue.Type, catalogue.Events, catalogue.OpenAPISpec,
		catalogue.CreatedAt, catalogue.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return ErrEventCatalogueExists
		}
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEventCatalogueNotCreated
	}

	cacheKey := convoy.EventCatalogueCacheKey.Get(catalogue.UID).String()
	err = e.cache.Set(ctx, cacheKey, catalogue, config.DefaultCacheTTL)
	if err != nil {
		return err
	}

	return nil
}

func (e *eventCatalogueRepo) UpdateEventCatalogue(ctx context.Context, catalogue *datastore.EventCatalogue) error {
	result, err := e.db.ExecContext(ctx, updateCatalogue,
		catalogue.Events, catalogue.OpenAPISpec,
		catalogue.UID, catalogue.ProjectID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEventCatalogueNotUpdated
	}

	cacheKey := convoy.EventCatalogueCacheKey.Get(catalogue.UID).String()
	return e.cache.Set(ctx, cacheKey, catalogue, config.DefaultCacheTTL)
}

func (e *eventCatalogueRepo) FindEventCatalogueByProjectID(ctx context.Context, projectID string) (*datastore.EventCatalogue, error) {
	fromCache, err := e.readFromCache(ctx, projectID, func() (*datastore.EventCatalogue, error) {
		catalogue := &datastore.EventCatalogue{}
		err := e.db.QueryRowxContext(ctx, fetchCatalogue, projectID).StructScan(catalogue)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, datastore.ErrCatalogueNotFound
			}
			return nil, err
		}

		return catalogue, nil
	})
	if err != nil {
		return nil, err
	}

	return fromCache, nil
}

func (e *eventCatalogueRepo) DeleteEventCatalogue(ctx context.Context, id, projectID string) error {
	result, err := e.db.ExecContext(ctx, deleteCatalogue, id, projectID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrAPIKeyNotRevoked
	}

	cacheKey := convoy.EventCatalogueCacheKey.Get(id).String()
	return e.cache.Delete(ctx, cacheKey)
}

func NewEventCatalogueRepo(db database.Database, cache cache.Cache) datastore.EventCatalogueRepository {
	return &eventCatalogueRepo{db: db.GetDB(), cache: cache}
}

func (a *eventCatalogueRepo) readFromCache(ctx context.Context, id string, readFromDB func() (*datastore.EventCatalogue, error)) (*datastore.EventCatalogue, error) {
	var catalogue *datastore.EventCatalogue
	cacheKey := convoy.EventCatalogueCacheKey.Get(id).String()
	err := a.cache.Get(ctx, cacheKey, &catalogue)
	if err != nil {
		return nil, err
	}

	if catalogue != nil {
		return catalogue, err
	}

	fromDB, err := readFromDB()
	if err != nil {
		return nil, err
	}

	err = a.cache.Set(ctx, cacheKey, fromDB, config.DefaultCacheTTL)
	if err != nil {
		return nil, err
	}

	return fromDB, err
}
