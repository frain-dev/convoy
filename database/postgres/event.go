package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/frain-dev/convoy/config"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
)

const (
	PartitionSize = 30_000

	createEvent = `
	INSERT INTO convoy.events (id,event_type,endpoints,project_id,
	                           source_id,headers,raw,data,url_query_params,
	                           idempotency_key,is_duplicate_event,acknowledged_at,metadata,status)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	updateEventEndpoints = `
	UPDATE convoy.events SET endpoints=$1 WHERE project_id= $2 AND id=$3
	`
	updateEventStatus = `
	UPDATE convoy.events SET status=$1 WHERE project_id= $2 AND id=$3
	`

	createEventEndpoints = `
	INSERT INTO convoy.events_endpoints (endpoint_id, event_id) VALUES (:endpoint_id, :event_id)
	ON CONFLICT (endpoint_id, event_id) DO NOTHING
	`

	fetchEventById = `
	SELECT id, event_type, endpoints, project_id,
    raw, data, headers, is_duplicate_event,
	COALESCE(source_id, '') AS source_id,
	COALESCE(idempotency_key, '') AS idempotency_key,
	COALESCE(url_query_params, '') AS url_query_params,
	created_at,updated_at,acknowledged_at,metadata,status
	FROM convoy.events WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	fetchEventsByIdempotencyKey = `
	SELECT id FROM convoy.events WHERE idempotency_key = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	fetchFirstEventWithIdempotencyKey = `
	SELECT id FROM convoy.events
	WHERE idempotency_key = $1
	AND is_duplicate_event IS FALSE
    AND project_id = $2
    AND deleted_at IS NULL
	ORDER BY created_at
	LIMIT 1;
	`

	fetchEventsByIds = `
	SELECT ev.id, ev.project_id,
    ev.is_duplicate_event, ev.id AS event_type,
	COALESCE(ev.source_id, '') AS source_id,
	COALESCE(ev.idempotency_key, '') AS idempotency_key,
	COALESCE(ev.url_query_params, '') AS url_query_params,
	ev.headers, ev.raw, ev.data, ev.created_at,
	ev.updated_at, ev.deleted_at,ev.acknowledged_at,
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
    SELECT COUNT(project_id) FROM convoy.events WHERE project_id = $1 AND deleted_at IS NULL;
	`
	countEvents = `
	SELECT COUNT(DISTINCT(ev.id)) FROM convoy.events ev
	LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
	LEFT JOIN convoy.endpoints e ON ee.endpoint_id = e.id
	WHERE ev.project_id = :project_id
	AND ev.created_at >= :start_date AND ev.created_at <= :end_date AND ev.deleted_at IS NULL;
	`

	baseEventsPaged = `
	SELECT ev.id, ev.project_id,
	ev.id AS event_type, ev.is_duplicate_event,
	COALESCE(ev.source_id, '') AS source_id,
	ev.headers, ev.raw, ev.data, ev.created_at,
	COALESCE(idempotency_key, '') AS idempotency_key,
	COALESCE(url_query_params, '') AS url_query_params,
	ev.updated_at, ev.deleted_at,ev.acknowledged_at,
	COALESCE(s.id, '') AS "source_metadata.id",
	COALESCE(s.name, '') AS "source_metadata.name"
    FROM convoy.events ev
	LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
	LEFT JOIN convoy.endpoints e ON e.id = ee.endpoint_id
	LEFT JOIN convoy.sources s ON s.id = ev.source_id
    WHERE ev.deleted_at IS NULL`

	baseEventsSearch = `
	SELECT ev.id, ev.project_id,
	ev.id AS event_type, ev.is_duplicate_event,
	COALESCE(ev.source_id, '') AS source_id,
	ev.headers, ev.raw, ev.data, ev.created_at,
	COALESCE(idempotency_key, '') AS idempotency_key,
	COALESCE(url_query_params, '') AS url_query_params,
	ev.updated_at, ev.deleted_at,
	COALESCE(s.id, '') AS "source_metadata.id",
	COALESCE(s.name, '') AS "source_metadata.name"
    FROM convoy.events_search ev
	LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
	LEFT JOIN convoy.endpoints e ON e.id = ee.endpoint_id
	LEFT JOIN convoy.sources s ON s.id = ev.source_id
    WHERE ev.deleted_at IS NULL`

	baseEventsPagedForward = `
	WITH events AS (
        %s %s AND ev.id <= :cursor
	    ORDER BY ev.created_at %s
	    LIMIT :limit
	)

	SELECT * FROM events ORDER BY created_at %s
	`

	baseEventsPagedBackward = `
	WITH events AS (
        %s %s AND ev.id >= :cursor
		ORDER BY ev.created_at %s
		LIMIT :limit
	)

	SELECT * FROM events ORDER BY created_at %s
	`

	baseEventFilter = ` AND ev.project_id = :project_id
	AND (ev.idempotency_key = :idempotency_key OR :idempotency_key = '')
	AND ev.created_at >= :start_date
	AND ev.created_at <= :end_date`

	endpointFilter = ` AND ee.endpoint_id IN (:endpoint_ids) `

	sourceFilter = ` AND ev.source_id IN (:source_ids) `

	searchFilter = ` AND search_token @@ websearch_to_tsquery('simple',:query) `

	baseCountPrevEvents = `
	select exists(
		SELECT 1
		FROM convoy.events ev
		LEFT JOIN convoy.events_endpoints ee ON ev.id = ee.event_id
		WHERE ev.deleted_at IS NULL
	`

	baseCountPrevEventSearch = `
	select exists(
		SELECT 1
		FROM convoy.events_search ev
		LEFT JOIN convoy.events_endpoints ee ON ev.id = ee.event_id
		WHERE ev.deleted_at IS NULL
	`
	countPrevEvents = ` AND ev.id > :cursor ORDER BY ev.created_at %s`

	softDeleteProjectEvents = `
	UPDATE convoy.events SET deleted_at = NOW()
	WHERE project_id = $1 AND created_at >= $2 AND created_at <= $3
	AND deleted_at IS NULL
	`

	hardDeleteProjectEvents = `
	DELETE FROM convoy.events WHERE project_id = $1 AND created_at >= $2 AND created_at <= $3
    AND NOT EXISTS (
    SELECT 1
    FROM convoy.event_deliveries
    WHERE event_id = convoy.events.id
    )
	`

	hardDeleteTokenizedEvents = `
	DELETE FROM convoy.events_search
    WHERE project_id = $1
	`

	copyRowsFromEventsToEventsSearch = `
    SELECT convoy.copy_rows($1, $2)
    `
)

type eventRepo struct {
	db database.Database
}

func NewEventRepo(db database.Database) datastore.EventRepository {
	return &eventRepo{db: db}
}

func (e *eventRepo) CreateEvent(ctx context.Context, event *datastore.Event) error {
	var sourceID *string

	if !util.IsStringEmpty(event.SourceID) {
		sourceID = &event.SourceID
	}
	event.Status = datastore.PendingStatus

	tx, isWrapped, err := GetTx(ctx, e.db.GetDB())
	if err != nil {
		return err
	}

	if !isWrapped {
		defer rollbackTx(tx)
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
		event.URLQueryParams,
		event.IdempotencyKey,
		event.IsDuplicateEvent,
		event.AcknowledgedAt,
		event.Metadata,
		event.Status,
	)
	if err != nil {
		return err
	}

	endpoints := event.Endpoints
	var j int
	for i := 0; i < len(endpoints); i += PartitionSize {
		j += PartitionSize
		if j > len(endpoints) {
			j = len(endpoints)
		}

		var ids []interface{}
		for _, endpointID := range endpoints[i:j] {
			ids = append(ids, &EventEndpoint{EventID: event.UID, EndpointID: endpointID})
		}

		_, err = tx.NamedExecContext(ctx, createEventEndpoints, ids)
		if err != nil {
			return err
		}
	}

	if isWrapped {
		return nil
	}

	return tx.Commit()
}

func (e *eventRepo) UpdateEventEndpoints(ctx context.Context, event *datastore.Event, endpoints []string) error {
	tx, isWrapped, err := GetTx(ctx, e.db.GetDB())
	if err != nil {
		return err
	}

	if !isWrapped {
		defer rollbackTx(tx)
	}

	_, err = tx.ExecContext(ctx, updateEventEndpoints,
		event.Endpoints,
		event.ProjectID,
		event.UID,
	)
	if err != nil {
		return err
	}

	var j int
	for i := 0; i < len(endpoints); i += PartitionSize {
		j += PartitionSize
		if j > len(endpoints) {
			j = len(endpoints)
		}

		var ids []interface{}
		for _, endpointID := range endpoints[i:j] {
			ids = append(ids, &EventEndpoint{EventID: event.UID, EndpointID: endpointID})
		}

		_, err = tx.NamedExecContext(ctx, createEventEndpoints, ids)
		if err != nil {
			return err
		}
	}

	if isWrapped {
		return nil
	}

	return tx.Commit()
}

func (e *eventRepo) UpdateEventStatus(ctx context.Context, event *datastore.Event, status datastore.EventStatus) error {
	tx, isWrapped, err := GetTx(ctx, e.db.GetDB())
	if err != nil {
		return err
	}

	if !isWrapped {
		defer rollbackTx(tx)
	}

	_, err = tx.ExecContext(ctx, updateEventStatus,
		status,
		event.ProjectID,
		event.UID,
	)
	if err != nil {
		return err
	}

	if isWrapped {
		return nil
	}

	return tx.Commit()
}

// FindEventByID to find events in real time - requires the primary db
func (e *eventRepo) FindEventByID(ctx context.Context, projectID string, id string) (*datastore.Event, error) {
	event := &datastore.Event{}
	err := e.db.GetDB().QueryRowxContext(ctx, fetchEventById, id, projectID).StructScan(event)
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

	query = e.db.GetReadDB().Rebind(query)
	rows, err := e.db.GetReadDB().QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer closeWithError(rows)

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

func (e *eventRepo) FindEventsByIdempotencyKey(ctx context.Context, projectID string, idempotencyKey string) ([]datastore.Event, error) {
	query, args, err := sqlx.In(fetchEventsByIdempotencyKey, idempotencyKey, projectID)
	if err != nil {
		return nil, err
	}

	query = e.db.GetDB().Rebind(query)
	rows, err := e.db.GetDB().QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer closeWithError(rows)

	events := make([]datastore.Event, 0)
	for rows.Next() {
		var event datastore.Event

		err = rows.StructScan(&event)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, nil
}

func (e *eventRepo) FindFirstEventWithIdempotencyKey(ctx context.Context, projectID string, id string) (*datastore.Event, error) {
	event := &datastore.Event{}
	err := e.db.GetDB().QueryRowxContext(ctx, fetchFirstEventWithIdempotencyKey, id, projectID).StructScan(event)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrEventNotFound
		}

		return nil, err
	}
	return event, nil
}

func (e *eventRepo) CountProjectMessages(ctx context.Context, projectID string) (int64, error) {
	var c int64

	err := e.db.GetReadDB().QueryRowxContext(ctx, countProjectMessages, projectID).Scan(&c)
	if err != nil {
		return c, err
	}

	return c, nil
}

func (e *eventRepo) CountEvents(ctx context.Context, projectID string, filter *datastore.Filter) (int64, error) {
	var eventsCount int64
	startDate, endDate := getCreatedDateFilter(filter.SearchParams.CreatedAtStart, filter.SearchParams.CreatedAtEnd)

	arg := map[string]interface{}{
		"endpoint_ids": filter.EndpointIDs,
		"project_id":   projectID,
		"source_id":    filter.SourceID,
		"start_date":   startDate,
		"end_date":     endDate,
	}

	query := countEvents
	if len(filter.EndpointIDs) > 0 {
		query += ` AND e.id IN (:endpoint_ids) `
	}

	if !util.IsStringEmpty(filter.SourceID) {
		query += ` AND ev.source_id = :source_id `
	}

	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return 0, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return 0, err
	}

	query = e.db.GetReadDB().Rebind(query)
	err = e.db.GetReadDB().QueryRowxContext(ctx, query, args...).Scan(&eventsCount)
	if err != nil {
		return eventsCount, err
	}

	return eventsCount, nil
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
		"endpoint_ids":    filter.EndpointIDs,
		"project_id":      projectID,
		"source_ids":      filter.SourceIDs,
		"limit":           filter.Pageable.Limit(),
		"start_date":      startDate,
		"end_date":        endDate,
		"query":           filter.Query,
		"cursor":          filter.Pageable.Cursor(),
		"idempotency_key": filter.IdempotencyKey,
	}

	base := baseEventsPaged
	var baseQueryPagination string
	if filter.Pageable.Direction == datastore.Next {
		baseQueryPagination = getFwdEventPageQuery(filter.Pageable.SortOrder())
	} else {
		baseQueryPagination = getBackwardEventPageQuery(filter.Pageable.SortOrder())
	}

	filterQuery = baseEventFilter

	if len(filter.SourceIDs) > 0 {
		filterQuery += sourceFilter
	}

	if len(filter.EndpointIDs) > 0 {
		filterQuery += endpointFilter
	}

	if !util.IsStringEmpty(filter.Query) {
		filterQuery += searchFilter
		base = baseEventsSearch
	}

	preOrder := filter.Pageable.SortOrder()
	if filter.Pageable.Direction == datastore.Prev {
		preOrder = reverseOrder(preOrder)
	}

	query = fmt.Sprintf(baseQueryPagination, base, filterQuery, preOrder, filter.Pageable.SortOrder())
	query, args, err = sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = e.db.GetReadDB().Rebind(query)
	rows, err := e.db.GetReadDB().QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer closeWithError(rows)

	events := make([]datastore.Event, 0)
	for rows.Next() {
		var data datastore.Event

		err = rows.StructScan(&data)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		events = append(events, data)
	}

	var rowCount datastore.PrevRowCount
	if len(events) > 0 {
		first := events[0]
		qarg := arg
		qarg["cursor"] = first.UID

		baseCountEvents := baseCountPrevEvents
		if !util.IsStringEmpty(filter.Query) {
			baseCountEvents = baseCountPrevEventSearch
		}

		tmp := getCountDeliveriesPrevRowQuery(filter.Pageable.SortOrder())
		tmp = fmt.Sprintf(tmp, filter.Pageable.SortOrder())

		cq := baseCountEvents + filterQuery + tmp + ");"
		countQuery, qargs, err = sqlx.Named(cq, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		countQuery, qargs, err = sqlx.In(countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = e.db.GetReadDB().Rebind(countQuery)

		// count the row number before the first row
		rows, err = e.db.GetReadDB().QueryxContext(ctx, countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		defer closeWithError(rows)

		if rows.Next() {
			err = rows.StructScan(&rowCount)
			if err != nil {
				return nil, datastore.PaginationData{}, err
			}
		}
	}

	ids := make([]string, len(events))
	for i := range events {
		ids[i] = events[i].UID
	}

	if len(events) > filter.Pageable.PerPage {
		events = events[:len(events)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: rowCount}
	pagination = pagination.Build(filter.Pageable, ids)

	return events, *pagination, nil
}

func (e *eventRepo) DeleteProjectEvents(ctx context.Context, projectID string, filter *datastore.EventFilter, hardDelete bool) error {
	query := softDeleteProjectEvents
	startDate, endDate := getCreatedDateFilter(filter.CreatedAtStart, filter.CreatedAtEnd)

	if hardDelete {
		query = hardDeleteProjectEvents
	}

	_, err := e.db.GetDB().ExecContext(ctx, query, projectID, startDate, endDate)
	if err != nil {
		return err
	}

	return nil
}

func (e *eventRepo) DeleteProjectTokenizedEvents(ctx context.Context, projectID string, filter *datastore.EventFilter) error {
	startDate, endDate := getCreatedDateFilter(filter.CreatedAtStart, filter.CreatedAtEnd)

	query := hardDeleteTokenizedEvents + " AND created_at >= $2 AND created_at <= $3"

	_, err := e.db.GetDB().ExecContext(ctx, query, projectID, startDate, endDate)
	if err != nil {
		return err
	}

	return nil
}

func (e *eventRepo) CopyRows(ctx context.Context, projectID string, interval int) error {
	tx, err := e.db.GetDB().BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	if interval != config.DefaultSearchTokenizationInterval {
		_, err = tx.ExecContext(ctx, hardDeleteTokenizedEvents, projectID)
		if err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, copyRowsFromEventsToEventsSearch, projectID, interval)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (e *eventRepo) ExportRecords(ctx context.Context, projectID string, createdAt time.Time, w io.Writer) (int64, error) {
	return exportRecords(ctx, e.db.GetReadDB(), "convoy.events", projectID, createdAt, w)
}

func getCreatedDateFilter(startDate, endDate int64) (time.Time, time.Time) {
	return time.Unix(startDate, 0), time.Unix(endDate, 0)
}

type EventEndpoint struct {
	EventID    string `db:"event_id"`
	EndpointID string `db:"endpoint_id"`
}

func getFwdEventPageQuery(sortOrder string) string {
	if sortOrder == "ASC" {
		return strings.Replace(baseEventsPagedForward, "<=", ">=", 1)
	}

	return baseEventsPagedForward
}

func getBackwardEventPageQuery(sortOrder string) string {
	if sortOrder == "ASC" {
		return strings.Replace(baseEventsPagedBackward, ">=", "<=", 1)
	}

	return baseEventsPagedBackward
}

func getCountDeliveriesPrevRowQuery(sortOrder string) string {
	if sortOrder == "ASC" {
		return strings.Replace(countPrevEvents, ">", "<", 1)
	}

	return countPrevEvents
}

func (e *eventRepo) PartitionEventsTable(ctx context.Context) error {
	_, err := e.db.GetDB().ExecContext(ctx, partitionEventsTable)
	if err != nil {
		return err
	}

	return nil
}

func (e *eventRepo) UnPartitionEventsTable(ctx context.Context) error {
	_, err := e.db.GetDB().ExecContext(ctx, unPartitionEventsTable)
	if err != nil {
		return err
	}

	return nil
}

var partitionEventsTable = `
CREATE OR REPLACE FUNCTION convoy.enforce_event_fk() 
    RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM convoy.events
        WHERE id = NEW.event_id
    ) THEN
        RAISE EXCEPTION 'Foreign key violation: event_id % does not exist in events', NEW.event_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION convoy.partition_events_table() 
    RETURNS VOID AS $$
DECLARE
    r RECORD;
BEGIN
    RAISE NOTICE 'Creating partitioned table...';

    -- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.events_new;

    -- Create partitioned table
    CREATE TABLE convoy.events_new (
        id                 VARCHAR NOT NULL,
        event_type         TEXT NOT NULL,
        endpoints          TEXT,
        project_id         VARCHAR NOT NULL REFERENCES convoy.projects,
        source_id          VARCHAR REFERENCES convoy.sources,
        headers            JSONB,
        raw                TEXT NOT NULL,
        data               BYTEA NOT NULL,
        created_at         TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
        updated_at         TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
        deleted_at         TIMESTAMPTZ,
        url_query_params   VARCHAR,
        idempotency_key    TEXT,
        is_duplicate_event BOOLEAN DEFAULT FALSE,
        acknowledged_at    TIMESTAMPTZ,
        status             TEXT,
        metadata           TEXT,
        PRIMARY KEY (id, created_at, project_id)
    ) PARTITION BY RANGE (project_id, created_at);

    RAISE NOTICE 'Creating partitions...';
    FOR r IN
        WITH dates AS (
            SELECT project_id, created_at::DATE
            FROM convoy.events
            GROUP BY created_at::DATE, project_id
        )
        SELECT project_id,
               created_at::TEXT AS start_date,
               (created_at + 1)::TEXT AS stop_date,
               'events_' || pg_catalog.REPLACE(project_id::TEXT, '-', '') || '_' || pg_catalog.REPLACE(created_at::TEXT, '-', '') AS partition_table_name
        FROM dates
    LOOP
        EXECUTE FORMAT(
            'CREATE TABLE IF NOT EXISTS convoy.%s PARTITION OF convoy.events_new FOR VALUES FROM (%L, %L) TO (%L, %L)',
            r.partition_table_name, r.project_id, r.start_date, r.project_id, r.stop_date
        );
    END LOOP;

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.events_new (
        id, event_type, endpoints, project_id, source_id, headers, raw, data,
        created_at, updated_at, deleted_at, url_query_params, idempotency_key,
        is_duplicate_event, acknowledged_at, status, metadata
    )
    SELECT id, event_type, endpoints, project_id, source_id, headers, raw, data,
           created_at, updated_at, deleted_at, url_query_params, idempotency_key,
           is_duplicate_event, acknowledged_at, status, metadata
    FROM convoy.events;

    -- Manage table renaming
    ALTER TABLE convoy.event_deliveries DROP CONSTRAINT IF EXISTS event_deliveries_event_id_fkey;
    ALTER TABLE convoy.events RENAME TO events_old;
    ALTER TABLE convoy.events_new RENAME TO events;
    DROP TABLE IF EXISTS convoy.events_old;

    RAISE NOTICE 'Recreating indexes...';
    CREATE INDEX idx_events_id_key ON convoy.events (id);
    CREATE INDEX idx_events_created_at_key ON convoy.events (created_at);
    CREATE INDEX idx_events_deleted_at_key ON convoy.events (deleted_at);
    CREATE INDEX idx_events_project_id_deleted_at_key ON convoy.events (project_id, deleted_at);
    CREATE INDEX idx_events_project_id_key ON convoy.events (project_id);
    CREATE INDEX idx_events_project_id_source_id ON convoy.events (project_id, source_id);
    CREATE INDEX idx_events_source_id ON convoy.events (source_id);
    CREATE INDEX idx_idempotency_key_key ON convoy.events (idempotency_key);
    CREATE INDEX idx_project_id_on_not_deleted ON convoy.events (project_id) WHERE deleted_at IS NULL;

    -- Recreate FK using trigger
    CREATE OR REPLACE TRIGGER event_fk_check
    BEFORE INSERT ON convoy.event_deliveries
    FOR EACH ROW EXECUTE FUNCTION convoy.enforce_event_fk();

    RAISE NOTICE 'Migration complete!';
END;
$$ LANGUAGE plpgsql;
select convoy.partition_events_table()
`

var unPartitionEventsTable = `
create or replace function convoy.un_partition_events_table() 
    returns VOID as $$
begin
	RAISE NOTICE 'Starting un-partitioning of events table...';
    
	-- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.events_new;

    -- Create partitioned table
    CREATE TABLE convoy.events_new
    (
        id                 VARCHAR not null primary key ,
        event_type         TEXT not null,
        endpoints          TEXT,
        project_id         VARCHAR not null
            constraint events_new_project_id_fkey
                references convoy.projects,
        source_id          VARCHAR
            constraint events_new_source_id_fkey
                references convoy.sources,
        headers            jsonb,
        raw                TEXT not null,
        data               bytea not null,
        created_at         TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP not null,
        updated_at         TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP,
        deleted_at         TIMESTAMP WITH TIME ZONE,
        url_query_params   VARCHAR,
        idempotency_key    TEXT,
        is_duplicate_event BOOLEAN default false,
        acknowledged_at    TIMESTAMP WITH TIME ZONE,
        status             TEXT,
        metadata           TEXT
    );

    RAISE NOTICE 'Migrating data...';
    insert into convoy.events_new select * from convoy.events;
    ALTER TABLE convoy.event_deliveries DROP CONSTRAINT if exists event_deliveries_event_id_fkey;
    ALTER TABLE convoy.event_deliveries
        ADD CONSTRAINT event_deliveries_event_id_fkey
            FOREIGN KEY (event_id) REFERENCES convoy.events_new (id);

    ALTER TABLE convoy.events RENAME TO events_old;
    ALTER TABLE convoy.events_new RENAME TO events;
    DROP TABLE IF EXISTS convoy.events_old;

    RAISE NOTICE 'Recreating indexes...';
    CREATE INDEX idx_events_created_at_key ON convoy.events (created_at);
    CREATE INDEX idx_events_deleted_at_key ON convoy.events (deleted_at);
    CREATE INDEX idx_events_project_id_deleted_at_key ON convoy.events (project_id, deleted_at);
    CREATE INDEX idx_events_project_id_key ON convoy.events (project_id);
    CREATE INDEX idx_events_project_id_source_id ON convoy.events (project_id, source_id);
    CREATE INDEX idx_events_source_id ON convoy.events (source_id);
    CREATE INDEX idx_idempotency_key_key ON convoy.events (idempotency_key);
    CREATE INDEX idx_project_id_on_not_deleted ON convoy.events (project_id) WHERE deleted_at IS NULL;
	RAISE NOTICE 'Successfully un-partitioned events table...';
end $$ language plpgsql;
select convoy.un_partition_events_table()
`
