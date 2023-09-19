package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
)

var ErrEventNotCreated = errors.New("event could not be created")

const (
	createEvent = `
	INSERT INTO convoy.events (id, event_type, endpoints, project_id, source_id, headers, raw, data,created_at,updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	createEventEndpoints = `
	INSERT INTO convoy.events_endpoints (endpoint_id, event_id) VALUES (:endpoint_id, :event_id)
	`

	fetchEventById = `
	SELECT id, event_type, endpoints, project_id,
	COALESCE(source_id, '') AS source_id, headers, raw, data
	FROM convoy.events WHERE id = $1 AND project_id = $2 AND deleted_at is NULL;
	`

	fetchEventsByIds = `
	SELECT ev.id, ev.project_id, ev.id as event_type,
	COALESCE(ev.source_id, '') AS source_id,
	ev.headers, ev.raw, ev.data, ev.created_at, 
	ev.updated_at, ev.deleted_at,
	COALESCE(s.id, '') AS "source_metadata.id",
	COALESCE(s.name, '') AS "source_metadata.name"
    FROM convoy.events ev
	LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
	LEFT JOIN convoy.endpoints e ON e.id = ee.endpoint_id
	LEFT JOIN convoy.sources s ON s.id = ev.source_id
	WHERE ev.deleted_at IS NULL
	AND ev.id IN (?) 
	AND ev.project_id = ?
	`

	countProjectMessages = `
	SELECT count(*) from convoy.events WHERE project_id = $1 AND deleted_at IS NULL;
	`
	countEvents = `
	SELECT count(distinct(ev.id)) from convoy.events ev
	LEFT JOIN convoy.events_endpoints ee on ee.event_id = ev.id
	LEFT JOIN convoy.endpoints e on ee.endpoint_id = e.id
	WHERE ev.project_id = $1 AND (e.id = $2 OR $2 = '' )
	AND (ev.source_id = $3 OR $3 = '') AND ev.created_at >= $4 AND ev.created_at <= $5 AND ev.deleted_at IS NULL;
	`

	baseEventsPaged = `
	SELECT ev.id, ev.project_id, ev.id as event_type,
	COALESCE(ev.source_id, '') AS source_id,
	ev.headers, ev.raw, ev.data, ev.created_at, 
	ev.updated_at, ev.deleted_at,
	COALESCE(s.id, '') AS "source_metadata.id",
	COALESCE(s.name, '') AS "source_metadata.name"
    FROM convoy.events ev
	LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
	LEFT JOIN convoy.endpoints e ON e.id = ee.endpoint_id
	LEFT JOIN convoy.sources s ON s.id = ev.source_id
    WHERE ev.deleted_at IS NULL`

	baseEventsPagedForward = `%s %s AND ev.id <= :cursor
	GROUP BY ev.id, s.id 
	ORDER BY ev.id DESC 
	LIMIT :limit
	`

	baseEventsPagedBackward = `
	WITH events AS (  
		%s %s AND ev.id >= :cursor 
		GROUP BY ev.id, s.id 
		ORDER BY ev.id ASC 
		LIMIT :limit
	)

	SELECT * FROM events ORDER BY id DESC
	`

	baseEventFilter = ` AND ev.project_id = :project_id 
	AND (ev.source_id = :source_id OR :source_id = '')
	AND (ev.id = :event_id OR :event_id = '') 
	AND ev.created_at >= :start_date 
	AND ev.created_at <= :end_date`

	endpointFilter = ` AND ee.endpoint_id IN (:endpoint_ids) `

	baseCountPrevEvents = `
	SELECT count(distinct(ev.id)) as count
	FROM convoy.events ev
	LEFT JOIN convoy.events_endpoints ee ON ev.id = ee.event_id
	WHERE ev.deleted_at IS NULL
	`
	countPrevEvents = ` AND ev.id > :cursor GROUP BY ev.id ORDER BY ev.id DESC LIMIT 1`

	softDeleteProjectEvents = `
	UPDATE convoy.events SET deleted_at = now()
	WHERE project_id = $1 AND created_at >= $2 AND created_at <= $3
	AND deleted_at IS NULL
	`
	hardDeleteProjectEvents = `
	DELETE from convoy.events WHERE project_id = $1 AND created_at
	>= $2 AND created_at <= $3 AND deleted_at IS NULL
	`
)

type eventRepo struct {
	db *sqlx.DB
}

func NewEventRepo(db database.Database) datastore.EventRepository {
	return &eventRepo{db: db.GetDB()}
}

func (e *eventRepo) CreateEvent(ctx context.Context, event *datastore.Event) error {
	var sourceID *string

	if !util.IsStringEmpty(event.SourceID) {
		sourceID = &event.SourceID
	}

	tx, err := e.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, createEvent,
		event.UID,
		event.EventType,
		event.Endpoints,
		event.ProjectID,
		sourceID,
		event.Headers,
		event.Raw,
		event.Data,
		event.CreatedAt, event.UpdatedAt,
	)
	if err != nil {
		return err
	}

	var ids []interface{}
	if len(event.Endpoints) > 0 {
		for _, endpointID := range event.Endpoints {
			ids = append(ids, &EventEndpoint{EventID: event.UID, EndpointID: endpointID})
		}

		_, err = tx.NamedExecContext(ctx, createEventEndpoints, ids)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (e *eventRepo) FindEventByID(ctx context.Context, projectID string, id string) (*datastore.Event, error) {
	event := &datastore.Event{}
	err := e.db.QueryRowxContext(ctx, fetchEventById, id, projectID).StructScan(event)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrEventNotFound
		}

		return nil, err
	}
	return event, nil
}

func (e *eventRepo) FindEventsByIDs(ctx context.Context, projectID string, ids []string) ([]datastore.Event, error) {
	query, args, err := sqlx.In(fetchEventsByIds, ids, projectID)
	if err != nil {
		return nil, err
	}

	query = e.db.Rebind(query)
	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	events := make([]datastore.Event, 0)
	for rows.Next() {
		var event datastore.Event

		err := rows.StructScan(&event)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, nil
}

func (e *eventRepo) CountProjectMessages(ctx context.Context, projectID string) (int64, error) {
	var count int64

	err := e.db.QueryRowxContext(ctx, countProjectMessages, projectID).Scan(&count)
	if err != nil {
		return count, err
	}

	return count, nil
}

func (e *eventRepo) CountEvents(ctx context.Context, projectID string, filter *datastore.Filter) (int64, error) {
	var count int64
	startDate, endDate := getCreatedDateFilter(filter.SearchParams.CreatedAtStart, filter.SearchParams.CreatedAtEnd)

	err := e.db.QueryRowxContext(ctx, countEvents, projectID, filter.EndpointID, filter.SourceID, startDate, endDate).Scan(&count)
	if err != nil {
		return count, err
	}

	return count, nil
}

func (e *eventRepo) LoadEventsPaged(ctx context.Context, projectID string, filter *datastore.Filter) ([]datastore.Event, datastore.PaginationData, error) {
	var query, countQuery, filterQuery string
	var err error
	var args, qargs []interface{}

	startDate, endDate := getCreatedDateFilter(filter.SearchParams.CreatedAtStart, filter.SearchParams.CreatedAtEnd)
	if !util.IsStringEmpty(filter.EndpointID) {
		filter.EndpointIDs = append(filter.EndpointIDs, filter.EndpointID)
	}

	arg := map[string]interface{}{
		"endpoint_ids": filter.EndpointIDs,
		"project_id":   projectID,
		"source_id":    filter.SourceID,
		"limit":        filter.Pageable.Limit(),
		"start_date":   startDate,
		"end_date":     endDate,
		"cursor":       filter.Pageable.Cursor(),
		"event_id":     filter.Query,
	}

	var baseQueryPagination string
	if filter.Pageable.Direction == datastore.Next {
		baseQueryPagination = baseEventsPagedForward
	} else {
		baseQueryPagination = baseEventsPagedBackward
	}

	filterQuery = baseEventFilter
	if len(filter.EndpointIDs) > 0 {
		filterQuery += endpointFilter
	}

	query = fmt.Sprintf(baseQueryPagination, baseEventsPaged, filterQuery)
	query, args, err = sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = e.db.Rebind(query)
	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	events := make([]datastore.Event, 0)
	for rows.Next() {
		var data datastore.Event

		err = rows.StructScan(&data)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		events = append(events, data)
	}

	var count datastore.PrevRowCount
	if len(events) > 0 {
		first := events[0]
		qarg := arg
		qarg["cursor"] = first.UID

		cq := baseCountPrevEvents + filterQuery + countPrevEvents
		countQuery, qargs, err = sqlx.Named(cq, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		countQuery, qargs, err = sqlx.In(countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = e.db.Rebind(countQuery)

		// count the row number before the first row
		rows, err := e.db.QueryxContext(ctx, countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		if rows.Next() {
			err = rows.StructScan(&count)
			if err != nil {
				return nil, datastore.PaginationData{}, err
			}
		}
		rows.Close()
	}

	ids := make([]string, len(events))
	for i := range events {
		ids[i] = events[i].UID
	}

	if len(events) > filter.Pageable.PerPage {
		events = events[:len(events)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(filter.Pageable, ids)

	return events, *pagination, rows.Close()
}

func (e *eventRepo) DeleteProjectEvents(ctx context.Context, projectID string, filter *datastore.EventFilter, hardDelete bool) error {
	query := softDeleteProjectEvents
	startDate, endDate := getCreatedDateFilter(filter.CreatedAtStart, filter.CreatedAtEnd)

	if hardDelete {
		query = hardDeleteProjectEvents
	}

	_, err := e.db.ExecContext(ctx, query, projectID, startDate, endDate)
	if err != nil {
		return err
	}

	return nil
}

func getCreatedDateFilter(startDate, endDate int64) (time.Time, time.Time) {
	return time.Unix(startDate, 0), time.Unix(endDate, 0)
}

type EventEndpoint struct {
	EventID    string `db:"event_id"`
	EndpointID string `db:"endpoint_id"`
}
