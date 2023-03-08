package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
	"gopkg.in/guregu/null.v4"
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

	baseEventsCount = `
	WITH table_count AS (
		SELECT count(distinct(ev.id)) as count
		FROM convoy.events ev LEFT JOIN convoy.events_endpoints
		ee ON ee.event_id = ev.id
		WHERE ev.deleted_at IS NULL
		%s
	)
	`

	baseEventsPaged = `
	SELECT ev.id, ev.project_id, ev.event_type,
	COALESCE(ev.source_id, '') AS source_id,
	ev.headers, ev.raw, ev.data, ev.created_at, 
	ev.updated_at, ev.deleted_at,
	array_to_json(ARRAY_AGG(json_build_object('uid', e.id, 'title', e.title, 'project_id', e.project_id, 'target_url', e.target_url))) AS endpoint_metadata,
	COALESCE(s.id, '') AS "source_metadata.id",
	COALESCE(s.name, '') AS "source_metadata.name"
    FROM convoy.events ev
	LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
	LEFT JOIN convoy.endpoints e ON e.id = ee.endpoint_id
	LEFT JOIN convoy.sources s ON s.id = ev.source_id
    WHERE ev.deleted_at IS NULL`

	baseEventFilter = ` AND ev.project_id = :project_id AND (ev.source_id = :source_id OR :source_id = '') AND ev.created_at >= :start_date AND ev.created_at <= :end_date group by ev.id, s.id order by ev.id desc`

	forwardPaginationQuery  = `AND ev.id < :cursor ORDER BY ev.id DESC LIMIT :limit`
	backwardPaginationQuery = `AND ev.id > :cursor ORDER BY ev.id DESC LIMIT :limit`

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
		"start_date":   startDate,
		"end_date":     endDate,
		"cursor":       filter.Pageable.Cursor(),
	}

	var pagingationQuery string
	if filter.Pageable.Direction == datastore.Next {
		pagingationQuery = forwardPaginationQuery
	} else {
		pagingationQuery = backwardPaginationQuery
	}

	if len(filter.EndpointIDs) > 0 {
		filterQuery := ` AND ee.endpoint_id IN (:endpoint_ids) ` + baseEventFilter
		q := baseEventsPaged + filterQuery + pagingationQuery
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
		q := baseEventsPaged + baseEventFilter + pagingationQuery
		query, args, err = sqlx.Named(q, arg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		query = e.db.Rebind(query)
	}

	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	eventMap := make(map[string]*datastore.Event)
	events := make([]datastore.Event, 0)
	for rows.Next() {
		var data EventPaginated

		err = rows.StructScan(&data)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		record, exists := eventMap[data.UID]
		if exists {
			endpoint := data.Endpoint
			if !util.IsStringEmpty(endpoint.UID.String) {
				record.EndpointMetadata = append(record.EndpointMetadata, getEventEndpoint(endpoint))
			}

		} else {
			event := getEvent(data)
			endpoint := data.Endpoint
			if !util.IsStringEmpty(endpoint.UID.String) {
				event.EndpointMetadata = append(event.EndpointMetadata, getEventEndpoint(endpoint))
			}

			source := data.Source
			if !util.IsStringEmpty(source.UID.String) {
				event.Source = &datastore.Source{
					UID:  source.UID.String,
					Name: source.Name.String,
				}
			}

			eventMap[event.UID] = event
		}
	}

	for _, event := range eventMap {
		events = append(events, *event)
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].UID > events[j].UID
	})

	var startCursor string
	if len(events) > 0 {
		startCursor = events[0].UID
	}

	var endCursor string
	if len(events) == filter.Pageable.PerPage {
		endCursor = events[filter.Pageable.PerPage-1].UID
	}

	return events, datastore.PaginationData{
		PerPage:         int64(filter.Pageable.PerPage),
		HasNextPage:     false,
		HasPreviousPage: false,
		StartCursor:     startCursor,
		EndCursor:       endCursor,
	}, nil
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

func getEvent(data EventPaginated) *datastore.Event {
	return &datastore.Event{
		UID:              data.UID,
		EventType:        datastore.EventType(data.EventType),
		SourceID:         data.SourceID.String,
		AppID:            data.AppID,
		ProjectID:        data.ProjectID,
		Headers:          data.Headers,
		Data:             data.Data,
		Raw:              data.Raw,
		EndpointMetadata: []*datastore.Endpoint{},
		CreatedAt:        data.CreatedAt,
		UpdatedAt:        data.UpdatedAt,
	}
}

func getEventEndpoint(endpoint *Endpoint) *datastore.Endpoint {
	return &datastore.Endpoint{
		UID:          endpoint.UID.String,
		Title:        endpoint.Title.String,
		ProjectID:    endpoint.ProjectID.String,
		SupportEmail: endpoint.SupportEmail.String,
		TargetURL:    endpoint.TargetUrl.String,
	}
}

type EventEndpoint struct {
	EventID    string `db:"event_id"`
	EndpointID string `db:"endpoint_id"`
}

type EventPaginated struct {
	Count     int                   `db:"count"`
	UID       string                `db:"id"`
	EventType string                `db:"event_type"`
	SourceID  null.String           `db:"source_id"`
	AppID     string                `db:"app_id"`
	ProjectID string                `db:"project_id"`
	Headers   httpheader.HTTPHeader `db:"headers"`
	Data      json.RawMessage       `db:"data"`
	Raw       string                `db:"raw"`
	CreatedAt time.Time             `db:"created_at"`
	UpdatedAt time.Time             `db:"updated_at"`
	DeletedAt null.Time             `db:"deleted_at"`
	Endpoint  *Endpoint             `db:"endpoint"`
	Source    *Source               `db:"source"`
}

type Endpoint struct {
	UID          null.String `db:"id"`
	Title        null.String `db:"title"`
	ProjectID    null.String `db:"project_id"`
	SupportEmail null.String `db:"support_email"`
	TargetUrl    null.String `db:"target_url"`
}

type Source struct {
	UID  null.String `db:"id"`
	Name null.String `db:"name"`
}
