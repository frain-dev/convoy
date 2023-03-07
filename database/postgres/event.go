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
	FROM convoy.events WHERE id = $1 AND deleted_at is NULL;
	`

	fetchEventsByIds = `
	SELECT id, event_type, endpoints, project_id,
	COALESCE(source_id, '') AS source_id, headers, raw, data
	FROM convoy.events WHERE id IN (?) AND deleted_at IS NULL;
	`

	countProjectMessages = `
	SELECT count(*) from convoy.events WHERE project_id = $1 AND deleted_at IS NULL;
	`
	countEvents = `
	SELECT count(distinct(ev.id)) from convoy.events ev
	LEFT JOIN convoy.events_endpoints ee on ee.event_id = ev.id
	LEFT JOIN convoy.endpoints e on ee.endpoint_id = e.id
	WHERE (ev.project_id = $1 OR $1 = '') AND (e.id = $2 OR $2 = '' )
	AND (ev.source_id = $3 OR $3 = '') AND ev.created_at >= $4 AND ev.created_at <= $5 AND ev.deleted_at IS NULL;
	`

	baseEventsPaged = `SELECT * from convoy.event_metadata`

	baseEventFilter = ` WHERE project_id = :project_id AND (source_id = :source_id OR :source_id = '') AND created_at >= :start_date AND created_at <= :end_date order by id desc LIMIT :limit OFFSET :offset`

	softDeleteProjectEvents = `
	UPDATE convoy.events SET deleted_at = now()
	WHERE project_id = $1 AND created_at >= $2 AND created_at <= $3
	AND deleted_at IS NULL
	`
	hardDeleteProjectEvents = `
	DELETE from convoy.events WHERE project_id = $1 AND created_at
	>= $2 AND created_at <= $3 AND deleted_at IS NULL
	`

	refreshEventMetdataView = `REFRESH MATERIALIZED VIEW CONCURRENTLY convoy.event_metadata`
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

func (e *eventRepo) FindEventByID(ctx context.Context, id string) (*datastore.Event, error) {
	event := &datastore.Event{}
	err := e.db.QueryRowxContext(ctx, fetchEventById, id).StructScan(event)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrEventNotFound
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
	var query, projectID string
	var err error
	var args []interface{}

	startDate, endDate := getCreatedDateFilter(filter.SearchParams.CreatedAtStart, filter.SearchParams.CreatedAtEnd)
	if filter.Project != nil {
		projectID = filter.Project.UID
	}

	if !util.IsStringEmpty(filter.EndpointID) {
		filter.EndpointIDs = append(filter.EndpointIDs, filter.EndpointID)
	}

	arg := map[string]interface{}{
		"endpoint_ids": filter.EndpointIDs,
		"project_id":   projectID,
		"source_id":    filter.SourceID,
		"limit":        filter.Pageable.Limit(),
		"offset":       filter.Pageable.Offset(),
		"start_date":   startDate,
		"end_date":     endDate,
	}

	if len(filter.EndpointIDs) > 0 {
		filterQuery := `AND ee.endpoint_id IN (:endpoint_ids) ` + baseEventFilter
		q := baseEventsPaged + filterQuery
		query, args, err = sqlx.Named(q, arg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		query, args, err = sqlx.In(query, args...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		query = e.db.Rebind(query)

	} else {
		q := baseEventsPaged + baseEventFilter
		query, args, err = sqlx.Named(q, arg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		query = e.db.Rebind(query)
	}

	fmt.Println("query is >>>>", query)

	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	count := 0
	events := make([]datastore.Event, 0)
	for rows.Next() {
		var data datastore.Event

		err = rows.StructScan(&data)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		events = append(events, data)
	}

	pagination := calculatePaginationData(count, filter.Pageable.Page, filter.Pageable.PerPage)
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
