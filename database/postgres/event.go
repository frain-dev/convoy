package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
)

var (
	ErrEventNotCreated = errors.New("event could not be created")
	ErrEventNotFound   = errors.New("event not found")
)

const (
	createEvent = `
	INSERT INTO convoy.events (id, event_type, project_id, source_id, headers, raw, data) 
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	createEventEndpoints = `
	INSERT INTO convoy.events_endpoints (endpoint_id, event_id) VALUES (:endpoint_id, :event_id)
	`

	fetchEventById = `
	SELECT * from convoy.events WHERE id = $1 AND deleted_at is NULL;
	`

	fetchEventsByIds = ` 
	SELECT * from convoy.events WHERE id IN (?) AND deleted_at IS NULL;
	`

	countProjectMessages = `
	SELECT count(*) from convoy.events WHERE project_id = $1 AND deleted_at IS NULL;
	`
	countEvents = `
	SELECT count(distinct(ev.id)) from convoy.events ev 
	LEFT JOIN convoy.events_endpoints ee on ee.event_id = ev.id 
	LEFT JOIN convoy.endpoints e on ee.endpoint_id = e.id
	WHERE (ev.project_id = $1 OR $1 = '') AND (e.id = $2 OR $2 = '' ) 
	AND (ev.source_id = $3 OR $3 = '') AND ev.created_at BETWEEN $4 AND $5 AND ev.deleted_at IS NULL;
	`
	baseEventsPaged = `
	SELECT count(*) OVER(), ev.id, ev.project_id, COALESCE(ev.source_id, '') AS source_id, ev.headers, ev.raw,
	ev.data, ev.created_at, ev.updated_at, ev.deleted_at, array_to_json(array_agg(json_build_object(
    'uid', e.id, 
	'title', e.title, 
	'project_id', e.project_id, 
	'support_email', e.support_email,
	'target_url', e.target_url, 
	'slack_webhook_url', e.slack_webhook_url, 
	'created_at', e.created_at, 
	'updated_at', e.updated_at, 
	'deleted_at', e.deleted_at))) AS endpoint_metadata,
	COALESCE(s.id, '') AS "source_metadata.id",
	COALESCE(s.name, '') AS "source_metadata.name"
	FROM convoy.events AS ev 
	LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
	LEFT JOIN convoy.endpoints e ON e.id = ee.endpoint_id
	LEFT JOIN convoy.sources s ON s.id = ev.source_id
	WHERE ev.deleted_at IS NULL
	`
	baseWhere = `AND (ev.project_id = ? OR ? = '') AND (ev.source_id = ? OR ? = '') AND ev.created_at BETWEEN ? AND ? GROUP BY ev.id, s.id LIMIT ? OFFSET ?`

	fetchEventsPaginatedFilterByEndpoints = baseEventsPaged + `AND e.id IN (?)` + baseWhere

	fetchEventsPaginated = baseEventsPaged + baseWhere

	softDeleteProjectEvents = `
	UPDATE convoy.events SET deleted_at = now()
	WHERE project_id = $1 AND created_at BETWEEN $2 AND $3 
	AND deleted_at IS NULL
	`
	hardDeleteProjectEvents = `
	DELETE from convoy.events WHERE project_id = $1 AND created_at
	BETWEEN $2 AND $3 AND deleted_at IS NULL
	`
)

type eventRepo struct {
	db *sqlx.DB
}

func NewEventRepo(db *sqlx.DB) datastore.EventRepository {
	return &eventRepo{db: db}
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

	headers, err := json.Marshal(event.Headers)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, createEvent,
		event.UID,
		event.EventType,
		event.ProjectID,
		sourceID,
		headers,
		event.Raw,
		event.Data,
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

func (e *eventRepo) FindEventByID(ctx context.Context, id string) (*datastore.Event, error) {
	event := &datastore.Event{}
	err := e.db.QueryRowxContext(ctx, fetchEventById, id).StructScan(event)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEventNotFound
		}

		return nil, err
	}
	return event, nil
}

func (e *eventRepo) FindEventsByIDs(ctx context.Context, ids []string) ([]datastore.Event, error) {
	query, args, err := sqlx.In(fetchEventsByIds, ids)
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

func (e *eventRepo) CountEvents(ctx context.Context, filter *datastore.Filter) (int64, error) {
	var count int64
	var projectID string
	startDate, endDate := getCreatedDateFilter(filter.SearchParams.CreatedAtStart, filter.SearchParams.CreatedAtEnd)

	if filter.Project != nil {
		projectID = filter.Project.UID
	}

	err := e.db.QueryRowxContext(ctx, countEvents, projectID, filter.EndpointID, filter.SourceID, startDate, endDate).Scan(&count)
	if err != nil {
		return count, err
	}

	return count, nil
}

func (e *eventRepo) LoadEventsPaged(ctx context.Context, filter *datastore.Filter) ([]datastore.Event, datastore.PaginationData, error) {
	var query string
	var args []interface{}
	var err error
	var projectID string
	startDate, endDate := getCreatedDateFilter(filter.SearchParams.CreatedAtStart, filter.SearchParams.CreatedAtEnd)

	if filter.Project != nil {
		projectID = filter.Project.UID
	}

	if !util.IsStringEmpty(filter.EndpointID) {
		filter.EndpointIDs = append(filter.EndpointIDs, filter.EndpointID)
	}

	if len(filter.EndpointIDs) > 0 {
		query, args, err = sqlx.In(fetchEventsPaginatedFilterByEndpoints, filter.EndpointIDs, projectID, projectID, filter.SourceID, filter.SourceID, startDate, endDate, filter.Pageable.Limit(), filter.Pageable.Offset())
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		query = e.db.Rebind(query)
	} else {
		query = e.db.Rebind(fetchEventsPaginated)
		args = []interface{}{projectID, projectID, filter.SourceID, filter.SourceID, startDate, endDate, filter.Pageable.Limit(), filter.Pageable.Offset()}
	}

	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	totalRecords := 0
	var events []datastore.Event
	for rows.Next() {
		var data EventPaginated

		err = rows.StructScan(&data)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		events = append(events, data.Event)
		totalRecords = data.Count
	}

	pagination := calculatePaginationData(totalRecords, filter.Pageable.Page, filter.Pageable.PerPage)
	return events, pagination, nil
}

func (e *eventRepo) DeleteProjectEvents(ctx context.Context, filter *datastore.EventFilter, hardDelete bool) error {
	query := softDeleteProjectEvents
	startDate, endDate := getCreatedDateFilter(filter.CreatedAtStart, filter.CreatedAtEnd)

	if hardDelete {
		query = hardDeleteProjectEvents
	}

	_, err := e.db.ExecContext(ctx, query, filter.ProjectID, startDate, endDate)
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

type EventPaginated struct {
	Count int
	datastore.Event
}
