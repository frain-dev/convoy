package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/compare"
	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
)

const (
	createFilter = `
    INSERT INTO convoy.filters (
	id, subscription_id, event_type,
	headers, body, raw_headers, raw_body
	)
    VALUES ($1, $2, $3, $4, $5, $6, $7);
    `

	updateFilter = `
    UPDATE convoy.filters SET
    headers=$2,
    body=$3,
    raw_headers=$4,
    raw_body=$5,
    updated_at=now()
    WHERE id = $1;
    `

	deleteFilter = `
    DELETE FROM convoy.filters
    WHERE id = $1;
    `

	findFilterByID = `
    SELECT
    id, subscription_id, event_type,
    headers, body, raw_headers, raw_body,
	created_at, updated_at
    FROM convoy.filters
    WHERE id = $1;
    `

	findFiltersBySubscriptionID = `
    SELECT
    id, subscription_id, event_type,
    headers, body, raw_headers, raw_body,
	created_at, updated_at
    FROM convoy.filters
    WHERE subscription_id = $1;
    `

	findFilterBySubscriptionAndEventType = `
    SELECT
    id, subscription_id, event_type,
    headers, body, raw_headers, raw_body,
    created_at, updated_at
    FROM convoy.filters
    WHERE subscription_id = $1
    AND event_type = $2;
    `
)

var (
	ErrFilterNotCreated = errors.New("filter could not be created")
	ErrFilterNotUpdated = errors.New("filter could not be updated")
	ErrFilterNotDeleted = errors.New("filter could not be deleted")
	ErrFilterNotFound   = errors.New("filter not found")
)

type filterRepo struct {
	db database.Database
}

func NewFilterRepo(db database.Database) datastore.FilterRepository {
	return &filterRepo{db: db}
}

func (f *filterRepo) CreateFilter(ctx context.Context, filter *datastore.EventTypeFilter) error {
	if util.IsStringEmpty(filter.UID) {
		filter.UID = ulid.Make().String()
	}

	if filter.CreatedAt.IsZero() {
		filter.CreatedAt = time.Now()
	}

	if filter.UpdatedAt.IsZero() {
		filter.UpdatedAt = time.Now()
	}

	err := filter.Body.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten body filter: %v", err)
	}

	err = filter.Headers.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten header filter: %v", err)
	}

	_, err = f.db.GetDB().ExecContext(
		ctx,
		createFilter,
		filter.UID,
		filter.SubscriptionID,
		filter.EventType,
		filter.Headers,
		filter.Body,
		filter.RawHeaders,
		filter.RawBody,
	)

	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create filter")
		return ErrFilterNotCreated
	}

	return nil
}

func (f *filterRepo) CreateFilters(ctx context.Context, filters []datastore.EventTypeFilter) error {
	tx, err := f.db.GetDB().BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i := range filters {
		filter := &filters[i]
		if util.IsStringEmpty(filter.UID) {
			filter.UID = ulid.Make().String()
		}

		if filter.CreatedAt.IsZero() {
			filter.CreatedAt = time.Now()
		}

		if filter.UpdatedAt.IsZero() {
			filter.UpdatedAt = time.Now()
		}

		err = filter.Body.Flatten()
		if err != nil {
			return fmt.Errorf("failed to flatten body filter: %v", err)
		}

		err = filter.Headers.Flatten()
		if err != nil {
			return fmt.Errorf("failed to flatten header filter: %v", err)
		}

		_, err = tx.ExecContext(
			ctx,
			createFilter,
			filter.UID,
			filter.SubscriptionID,
			filter.EventType,
			filter.Headers,
			filter.Body,
			filter.RawHeaders,
			filter.RawBody,
		)

		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to create filter")
			return ErrFilterNotCreated
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (f *filterRepo) UpdateFilter(ctx context.Context, filter *datastore.EventTypeFilter) error {
	err := filter.Body.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten body filter: %v", err)
	}

	err = filter.Headers.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten header filter: %v", err)
	}

	result, err := f.db.GetDB().ExecContext(
		ctx,
		updateFilter,
		filter.UID,
		filter.Headers,
		filter.Body,
		filter.RawHeaders,
		filter.RawBody,
	)

	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update filter")
		return ErrFilterNotUpdated
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to get rows affected")
		return ErrFilterNotUpdated
	}

	if rowsAffected == 0 {
		return ErrFilterNotFound
	}

	return nil
}

func (f *filterRepo) DeleteFilter(ctx context.Context, filterID string) error {
	result, err := f.db.GetDB().ExecContext(ctx, deleteFilter, filterID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to delete filter")
		return ErrFilterNotDeleted
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to get rows affected")
		return ErrFilterNotDeleted
	}

	if rowsAffected == 0 {
		return ErrFilterNotFound
	}

	return nil
}

func (f *filterRepo) FindFilterByID(ctx context.Context, filterID string) (*datastore.EventTypeFilter, error) {
	var filter datastore.EventTypeFilter
	err := f.db.GetDB().QueryRowxContext(ctx, findFilterByID, filterID).StructScan(&filter)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFilterNotFound
		}
		return nil, err
	}

	return &filter, nil
}

func (f *filterRepo) FindFiltersBySubscriptionID(ctx context.Context, subscriptionID string) ([]datastore.EventTypeFilter, error) {
	rows, err := f.db.GetDB().QueryxContext(ctx, findFiltersBySubscriptionID, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFilters(rows)
}

func (f *filterRepo) FindFilterBySubscriptionAndEventType(ctx context.Context, subscriptionID, eventType string) (*datastore.EventTypeFilter, error) {
	var filter datastore.EventTypeFilter
	err := f.db.GetDB().QueryRowxContext(ctx, findFilterBySubscriptionAndEventType, subscriptionID, eventType).StructScan(&filter)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFilterNotFound
		}
		return nil, err
	}

	return &filter, nil
}

func (f *filterRepo) TestFilter(ctx context.Context, subscriptionID, eventType string, payload interface{}) (bool, error) {
	filter, err := f.FindFilterBySubscriptionAndEventType(ctx, subscriptionID, eventType)
	if err != nil {
		if errors.Is(err, ErrFilterNotFound) {
			// If no filter exists for this event type, check for a catch-all filter
			_filter, _err := f.FindFilterBySubscriptionAndEventType(ctx, subscriptionID, "*")
			if _err != nil {
				if errors.Is(_err, ErrFilterNotFound) {
					// there is no filtering, so it matches
					return true, nil
				}
				return false, _err
			}

			filter = _filter
		} else {
			return false, err
		}
	}

	if len(filter.Body) == 0 {
		// Empty filter means it matches everything
		return true, nil
	}

	p, err := flatten.Flatten(payload)
	if err != nil {
		return false, err
	}

	return compare.Compare(p, filter.Body)
}

func scanFilters(rows *sqlx.Rows) ([]datastore.EventTypeFilter, error) {
	var filters []datastore.EventTypeFilter

	for rows.Next() {
		var filter datastore.EventTypeFilter
		err := rows.StructScan(&filter)
		if err != nil {
			return nil, err
		}

		filters = append(filters, filter)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return filters, nil
}
