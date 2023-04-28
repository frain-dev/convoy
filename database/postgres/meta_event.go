package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	baseMetaEventsPaged = `
	SELECT mv.id, mv.project_id, mv.event_type,
	mv.data, mv.status, mv.retry_count, mv.max_retry_count, 
	mv.created_at, mv.updated_at FROM convoy.meta_events mv
	`
	baseMetaEventsPagedForward = `%s %s AND mv.id <= :cursor
	ORDER BY mv.id DESC
	LIMIT :limit
	`
	baseMetaEventsPagedBackward = `
	WITH meta_events AS (
		%s %s AND mv.id >= :cursor
		ORDER BY mv.id ASC
		LIMIT :limit
	)

	SELECT * from meta_events ORDER BY id DESC
	`
	baseMetaEventFilter = ` AND mv.project_id = :project_id
	AND mv.created_at >= :start_date
	AND mv.created_at <= :end_date`

	baseCountPrevMetaEvents = `
	SELECT count(distinct(mv.id)) as count
	FROM convoy.meta_events mv WHERE mv.deleted_at IS NULL
	`
	countPrevMetaEvents = ` AND mv.id > :cursor ORDER BY mv.id DESC LIMIT 1`
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

func (m *metaEventRepo) LoadMetaEventsPaged(ctx context.Context, projectID string, filter *datastore.Filter) ([]datastore.MetaEvent, datastore.PaginationData, error) {
	var query, countQuery, filterQuery string
	var err error
	var args, qargs []interface{}

	startDate, endDate := getCreatedDateFilter(filter.SearchParams.CreatedAtStart, filter.SearchParams.CreatedAtEnd)

	arg := map[string]interface{}{
		"project_id": projectID,
		"start_date": startDate,
		"end_date":   endDate,
		"limit":      filter.Pageable.Limit(),
		"cursor":     filter.Pageable.Cursor(),
	}

	var baseQueryPagination string
	if filter.Pageable.Direction == datastore.Next {
		baseQueryPagination = baseMetaEventsPagedForward
	} else {
		baseQueryPagination = baseMetaEventsPagedBackward
	}

	filterQuery = baseMetaEventFilter
	query = fmt.Sprintf(baseQueryPagination, baseMetaEventsPaged, filterQuery)

	query, args, err = sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = m.db.Rebind(query)
	fmt.Println("query is >>>>", query)
	rows, err := m.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	metaEvents := make([]datastore.MetaEvent, 0)
	for rows.Next() {
		var data datastore.MetaEvent

		err = rows.StructScan(&data)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		metaEvents = append(metaEvents, data)
	}

	var count datastore.PrevRowCount
	if len(metaEvents) > 0 {
		first := metaEvents[0]
		qarg := arg
		qarg["cursor"] = first.UID

		cq := baseCountPrevMetaEvents + filterQuery + countPrevMetaEvents
		countQuery, qargs, err = sqlx.Named(cq, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = m.db.Rebind(countQuery)
		rows, err := m.db.QueryxContext(ctx, countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		if rows.Next() {
			err = rows.StructScan(&count)
		}
	}
	return nil, datastore.PaginationData{}, nil
}
