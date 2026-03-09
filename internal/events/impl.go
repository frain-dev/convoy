package events

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/events/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

const (
	PartitionSize = 30_000 // Batch size for event_endpoints inserts
)

// Service implements datastore.EventRepository using sqlc-generated queries
type Service struct {
	logger log.StdLogger
	repo   repo.Querier
	db     *pgxpool.Pool
}

// Ensure Service implements datastore.EventRepository at compile time
var _ datastore.EventRepository = (*Service)(nil)

// New creates a new events service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// CreateEvent inserts a new event with batch endpoint processing
func (s *Service) CreateEvent(ctx context.Context, event *datastore.Event) error {
	// Set default status
	event.Status = datastore.PendingStatus

	// Prepare source_id
	var sourceID *string
	if !util.IsStringEmpty(event.SourceID) {
		sourceID = &event.SourceID
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Create event params
	params := repo.CreateEventParams{
		ID:               common.StringToPgText(event.UID),
		EventType:        common.StringToPgText(string(event.EventType)),
		Endpoints:        common.StringToPgText(endpointsToString(event.Endpoints)),
		ProjectID:        common.StringToPgText(event.ProjectID),
		SourceID:         common.StringPtrToPgText(sourceID),
		Headers:          headersToJSONB(event.Headers),
		Raw:              common.StringToPgText(event.Raw),
		Data:             event.Data,
		UrlQueryParams:   pgtype.Text{String: event.URLQueryParams, Valid: true},
		IdempotencyKey:   common.StringToPgText(event.IdempotencyKey),
		IsDuplicateEvent: common.BoolToPgBool(event.IsDuplicateEvent),
		AcknowledgedAt:   common.NullTimeToPgTimestamptz(event.AcknowledgedAt),
		Metadata:         common.StringToPgText(event.Metadata),
		Status:           common.StringToPgText(string(event.Status)),
	}

	// Insert event
	err = qtx.CreateEvent(ctx, params)
	if err != nil {
		return err
	}

	// Batch insert event_endpoints in 30K partitions
	endpoints := event.Endpoints
	for i := 0; i < len(endpoints); i += PartitionSize {
		end := i + PartitionSize
		if end > len(endpoints) {
			end = len(endpoints)
		}

		for _, endpointID := range endpoints[i:end] {
			endpointParams := repo.CreateEventEndpointsParams{
				EventID:    common.StringToPgText(event.UID),
				EndpointID: common.StringToPgText(endpointID),
			}
			err = qtx.CreateEventEndpoints(ctx, endpointParams)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// FindEventByID finds an event by ID
func (s *Service) FindEventByID(ctx context.Context, projectID, id string) (*datastore.Event, error) {
	params := repo.FindEventByIDParams{
		ID:        common.StringToPgText(id),
		ProjectID: common.StringToPgText(projectID),
	}
	row, err := s.repo.FindEventByID(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrEventNotFound
		}
		return nil, err
	}

	return rowToEvent(row)
}

// FindEventsByIDs finds multiple events by IDs
func (s *Service) FindEventsByIDs(ctx context.Context, projectID string, ids []string) ([]datastore.Event, error) {
	params := repo.FindEventsByIDsParams{
		EventIds:  ids,
		ProjectID: common.StringToPgText(projectID),
	}
	rows, err := s.repo.FindEventsByIDs(ctx, params)
	if err != nil {
		return nil, err
	}

	events := make([]datastore.Event, 0, len(rows))
	for _, row := range rows {
		event, err := rowToEvent(row)
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}

	return events, nil
}

// FindEventsByIdempotencyKey finds events with a specific idempotency key
func (s *Service) FindEventsByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) ([]datastore.Event, error) {
	params := repo.FindEventsByIdempotencyKeyParams{
		IdempotencyKey: common.StringToPgText(idempotencyKey),
		ProjectID:      common.StringToPgText(projectID),
	}
	ids, err := s.repo.FindEventsByIdempotencyKey(ctx, params)
	if err != nil {
		return nil, err
	}

	events := make([]datastore.Event, 0, len(ids))
	for _, id := range ids {
		// These rows only have ID, need to fetch full event
		event, err := s.FindEventByID(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}

	return events, nil
}

// FindFirstEventWithIdempotencyKey finds the first non-duplicate event
func (s *Service) FindFirstEventWithIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (*datastore.Event, error) {
	params := repo.FindFirstEventWithIdempotencyKeyParams{
		IdempotencyKey: common.StringToPgText(idempotencyKey),
		ProjectID:      common.StringToPgText(projectID),
	}
	id, err := s.repo.FindFirstEventWithIdempotencyKey(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrEventNotFound
		}
		return nil, err
	}

	// Fetch full event details
	return s.FindEventByID(ctx, projectID, id)
}

// UpdateEventEndpoints updates event endpoints with batch processing
func (s *Service) UpdateEventEndpoints(ctx context.Context, event *datastore.Event, endpoints []string) error {
	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Update endpoints array
	updateParams := repo.UpdateEventEndpointsParams{
		Endpoints: common.StringToPgText(endpointsToString(endpoints)),
		ProjectID: common.StringToPgText(event.ProjectID),
		ID:        common.StringToPgText(event.UID),
	}
	err = qtx.UpdateEventEndpoints(ctx, updateParams)
	if err != nil {
		return err
	}

	// Batch insert new event_endpoints in 30K partitions
	for i := 0; i < len(endpoints); i += PartitionSize {
		end := i + PartitionSize
		if end > len(endpoints) {
			end = len(endpoints)
		}

		for _, endpointID := range endpoints[i:end] {
			createParams := repo.CreateEventEndpointsParams{
				EventID:    common.StringToPgText(event.UID),
				EndpointID: common.StringToPgText(endpointID),
			}
			err = qtx.CreateEventEndpoints(ctx, createParams)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// UpdateEventStatus updates event status
func (s *Service) UpdateEventStatus(ctx context.Context, event *datastore.Event, status datastore.EventStatus) error {
	params := repo.UpdateEventStatusParams{
		Status:    common.StringToPgText(string(status)),
		ProjectID: common.StringToPgText(event.ProjectID),
		ID:        common.StringToPgText(event.UID),
	}
	return s.repo.UpdateEventStatus(ctx, params)
}

// CountProjectMessages counts total events in a project
func (s *Service) CountProjectMessages(ctx context.Context, projectID string) (int64, error) {
	count, err := s.repo.CountProjectMessages(ctx, common.StringToPgText(projectID))
	if err != nil {
		return 0, err
	}
	return count.Int64, nil
}

// CountEvents counts events with filters
func (s *Service) CountEvents(ctx context.Context, projectID string, filter *datastore.Filter) (int64, error) {
	startDate, endDate := getCreatedDateFilter(filter.SearchParams.CreatedAtStart, filter.SearchParams.CreatedAtEnd)

	params := repo.CountEventsParams{
		ProjectID:      common.StringToPgText(projectID),
		StartDate:      common.TimeToPgTimestamptz(startDate),
		EndDate:        common.TimeToPgTimestamptz(endDate),
		HasEndpointIds: common.BoolToPgBool(len(filter.EndpointIDs) > 0),
		EndpointIds:    filter.EndpointIDs,
		HasSourceID:    common.BoolToPgBool(!util.IsStringEmpty(filter.SourceID)),
		SourceID:       common.StringToPgText(filter.SourceID),
	}

	count, err := s.repo.CountEvents(ctx, params)
	if err != nil {
		return 0, err
	}
	return count.Int64, nil
}

// LoadEventsPaged is the most complex method - handles bidirectional pagination with dual query paths
func (s *Service) LoadEventsPaged(ctx context.Context, projectID string, filter *datastore.Filter) ([]datastore.Event, datastore.PaginationData, error) {
	startDate, endDate := getCreatedDateFilter(filter.SearchParams.CreatedAtStart, filter.SearchParams.CreatedAtEnd)

	// Add single EndpointID to EndpointIDs array if present
	if !util.IsStringEmpty(filter.EndpointID) {
		filter.EndpointIDs = append(filter.EndpointIDs, filter.EndpointID)
	}

	// Decide query path: empty search query uses EXISTS path for better index usage
	useExistsPath := util.IsStringEmpty(filter.Query)

	var events []datastore.Event
	var err error

	if useExistsPath {
		// EXISTS path: Fast pagination without GROUP BY
		events, err = s.loadEventsPagedExists(ctx, projectID, filter, startDate, endDate)
	} else {
		// CTE path: Full-text search with GROUP BY
		events, err = s.loadEventsPagedSearch(ctx, projectID, filter, startDate, endDate)
	}

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	// Calculate PrevRowCount if not first page
	var rowCount datastore.PrevRowCount
	isFirstPage := util.IsStringEmpty(filter.Pageable.Cursor())
	if len(events) > 0 && !isFirstPage {
		first := events[0]
		rowCount, err = s.countPrevEvents(ctx, projectID, filter, first.UID, startDate, endDate, useExistsPath)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
	}

	// Build event IDs for pagination metadata
	ids := make([]string, len(events))
	for i := range events {
		ids[i] = events[i].UID
	}

	// Build pagination metadata with untrimmed ids (Build needs the extra item to detect hasNext)
	pagination := &datastore.PaginationData{PrevRowCount: rowCount}
	pagination = pagination.Build(filter.Pageable, ids)

	// Trim LIMIT+1 after building pagination (hasNext detection is done, now remove the extra item)
	if len(events) > filter.Pageable.PerPage {
		events = events[:len(events)-1]
	}

	return events, *pagination, nil
}

// loadEventsPagedExists handles EXISTS path pagination (no search query)
func (s *Service) loadEventsPagedExists(ctx context.Context, projectID string, filter *datastore.Filter, startDate, endDate time.Time) ([]datastore.Event, error) {
	// Determine cursor conditions based on direction and sort order
	hasCursor := !util.IsStringEmpty(filter.Pageable.Cursor())
	cursor := filter.Pageable.Cursor()
	sortAsc := filter.Pageable.SortOrder() == "ASC"

	// Cursor logic:
	// Forward + DESC: id <= cursor (cursorLte=true, cursorGte=false)
	// Forward + ASC: id >= cursor (cursorLte=false, cursorGte=true)
	// Backward + DESC: id >= cursor (cursorLte=false, cursorGte=true)
	// Backward + ASC: id <= cursor (cursorLte=true, cursorGte=false)
	var cursorLte, cursorGte bool
	if filter.Pageable.Direction == datastore.Next {
		if sortAsc {
			cursorGte = true // Forward + ASC: id >= cursor
		} else {
			cursorLte = true // Forward + DESC: id <= cursor
		}
	} else {
		if sortAsc {
			cursorLte = true // Backward + ASC: id <= cursor
		} else {
			cursorGte = true // Backward + DESC: id >= cursor
		}
	}

	params := repo.LoadEventsPagedExistsParams{
		HasEndpointOrOwnerFilter: common.BoolToPgBool(!util.IsStringEmpty(filter.OwnerID) || len(filter.EndpointIDs) > 0),
		HasOwnerID:               common.BoolToPgBool(!util.IsStringEmpty(filter.OwnerID)),
		OwnerID:                  common.StringToPgText(filter.OwnerID),
		HasEndpointIds:           common.BoolToPgBool(len(filter.EndpointIDs) > 0),
		EndpointIds:              filter.EndpointIDs,
		ProjectID:                common.StringToPgText(projectID),
		HasIdempotencyKey:        common.BoolToPgBool(!util.IsStringEmpty(filter.IdempotencyKey)),
		IdempotencyKey:           common.StringToPgText(filter.IdempotencyKey),
		StartDate:                common.TimeToPgTimestamptz(startDate),
		EndDate:                  common.TimeToPgTimestamptz(endDate),
		HasSourceIds:             common.BoolToPgBool(len(filter.SourceIDs) > 0),
		SourceIds:                filter.SourceIDs,
		HasBrokerMessageID:       common.BoolToPgBool(!util.IsStringEmpty(filter.BrokerMessageId)),
		BrokerMessageID:          common.StringToPgText(filter.BrokerMessageId),
		HasCursor:                common.BoolToPgBool(hasCursor && cursorLte),
		Cursor:                   common.StringToPgText(cursor),
		CursorGte:                common.BoolToPgBool(hasCursor && cursorGte),
		SortAsc:                  common.BoolToPgBool(sortAsc),
		PageLimit:                pgtype.Int8{Int64: int64(filter.Pageable.Limit()), Valid: true},
	}

	rows, err := s.repo.LoadEventsPagedExists(ctx, params)
	if err != nil {
		return nil, err
	}

	events := make([]datastore.Event, 0, len(rows))
	for _, row := range rows {
		event, err := rowToEvent(row)
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}

	return events, nil
}

// loadEventsPagedSearch handles CTE path pagination (with search query)
func (s *Service) loadEventsPagedSearch(ctx context.Context, projectID string, filter *datastore.Filter, startDate, endDate time.Time) ([]datastore.Event, error) {
	// Determine cursor conditions
	hasCursor := !util.IsStringEmpty(filter.Pageable.Cursor())
	cursor := filter.Pageable.Cursor()
	sortAsc := filter.Pageable.SortOrder() == "ASC"

	var cursorLte, cursorGte bool
	if filter.Pageable.Direction == datastore.Next {
		if sortAsc {
			cursorGte = true
		} else {
			cursorLte = true
		}
	} else {
		if sortAsc {
			cursorLte = true
		} else {
			cursorGte = true
		}
	}

	params := repo.LoadEventsPagedSearchParams{
		SortAsc:            common.BoolToPgBool(sortAsc),
		ProjectID:          common.StringToPgText(projectID),
		HasIdempotencyKey:  common.BoolToPgBool(!util.IsStringEmpty(filter.IdempotencyKey)),
		IdempotencyKey:     common.StringToPgText(filter.IdempotencyKey),
		StartDate:          common.TimeToPgTimestamptz(startDate),
		EndDate:            common.TimeToPgTimestamptz(endDate),
		HasSourceIds:       common.BoolToPgBool(len(filter.SourceIDs) > 0),
		SourceIds:          filter.SourceIDs,
		HasEndpointIds:     common.BoolToPgBool(len(filter.EndpointIDs) > 0),
		EndpointIds:        filter.EndpointIDs,
		HasBrokerMessageID: common.BoolToPgBool(!util.IsStringEmpty(filter.BrokerMessageId)),
		BrokerMessageID:    common.StringToPgText(filter.BrokerMessageId),
		HasQuery:           common.BoolToPgBool(!util.IsStringEmpty(filter.Query)),
		Query:              common.StringToPgText(filter.Query),
		HasCursor:          common.BoolToPgBool(hasCursor && cursorLte),
		Cursor:             common.StringToPgText(cursor),
		CursorGte:          common.BoolToPgBool(hasCursor && cursorGte),
		PageLimit:          pgtype.Int8{Int64: int64(filter.Pageable.Limit()), Valid: true},
	}

	rows, err := s.repo.LoadEventsPagedSearch(ctx, params)
	if err != nil {
		return nil, err
	}

	events := make([]datastore.Event, 0, len(rows))
	for _, row := range rows {
		event, err := rowToEvent(row)
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}

	return events, nil
}

// countPrevEvents checks if there are events before cursor (for HasPrevPage)
func (s *Service) countPrevEvents(ctx context.Context, projectID string, filter *datastore.Filter, cursor string, startDate, endDate time.Time, useExistsPath bool) (datastore.PrevRowCount, error) {
	sortAsc := filter.Pageable.SortOrder() == "ASC"

	if useExistsPath {
		params := repo.CountPrevEventsExistsParams{
			ProjectID:          common.StringToPgText(projectID),
			HasIdempotencyKey:  common.BoolToPgBool(!util.IsStringEmpty(filter.IdempotencyKey)),
			IdempotencyKey:     common.StringToPgText(filter.IdempotencyKey),
			StartDate:          common.TimeToPgTimestamptz(startDate),
			EndDate:            common.TimeToPgTimestamptz(endDate),
			HasSourceIds:       common.BoolToPgBool(len(filter.SourceIDs) > 0),
			SourceIds:          filter.SourceIDs,
			HasEndpointIds:     common.BoolToPgBool(len(filter.EndpointIDs) > 0),
			EndpointIds:        filter.EndpointIDs,
			HasBrokerMessageID: common.BoolToPgBool(!util.IsStringEmpty(filter.BrokerMessageId)),
			BrokerMessageID:    common.StringToPgText(filter.BrokerMessageId),
			SortAsc:            common.BoolToPgBool(sortAsc),
			Cursor:             common.StringToPgText(cursor),
		}

		exists, err := s.repo.CountPrevEventsExists(ctx, params)
		if err != nil {
			return datastore.PrevRowCount{}, err
		}
		count := 0
		if exists {
			count = 1
		}
		return datastore.PrevRowCount{Count: count}, nil
	}

	// Search path
	params := repo.CountPrevEventsSearchParams{
		ProjectID:          common.StringToPgText(projectID),
		HasIdempotencyKey:  common.BoolToPgBool(!util.IsStringEmpty(filter.IdempotencyKey)),
		IdempotencyKey:     common.StringToPgText(filter.IdempotencyKey),
		StartDate:          common.TimeToPgTimestamptz(startDate),
		EndDate:            common.TimeToPgTimestamptz(endDate),
		HasSourceIds:       common.BoolToPgBool(len(filter.SourceIDs) > 0),
		SourceIds:          filter.SourceIDs,
		HasEndpointIds:     common.BoolToPgBool(len(filter.EndpointIDs) > 0),
		EndpointIds:        filter.EndpointIDs,
		HasBrokerMessageID: common.BoolToPgBool(!util.IsStringEmpty(filter.BrokerMessageId)),
		BrokerMessageID:    common.StringToPgText(filter.BrokerMessageId),
		HasQuery:           common.BoolToPgBool(!util.IsStringEmpty(filter.Query)),
		Query:              common.StringToPgText(filter.Query),
		SortAsc:            common.BoolToPgBool(sortAsc),
		Cursor:             common.StringToPgText(cursor),
	}

	exists, err := s.repo.CountPrevEventsSearch(ctx, params)
	if err != nil {
		return datastore.PrevRowCount{}, err
	}
	count := 0
	if exists {
		count = 1
	}
	return datastore.PrevRowCount{Count: count}, nil
}

// DeleteProjectEvents soft or hard deletes events
func (s *Service) DeleteProjectEvents(ctx context.Context, projectID string, filter *datastore.EventFilter, hardDelete bool) error {
	startDate, endDate := getCreatedDateFilter(filter.CreatedAtStart, filter.CreatedAtEnd)

	if hardDelete {
		params := repo.HardDeleteProjectEventsParams{
			ProjectID: common.StringToPgText(projectID),
			StartDate: common.TimeToPgTimestamptz(startDate),
			EndDate:   common.TimeToPgTimestamptz(endDate),
		}
		return s.repo.HardDeleteProjectEvents(ctx, params)
	}

	params := repo.SoftDeleteProjectEventsParams{
		ProjectID: common.StringToPgText(projectID),
		StartDate: common.TimeToPgTimestamptz(startDate),
		EndDate:   common.TimeToPgTimestamptz(endDate),
	}
	return s.repo.SoftDeleteProjectEvents(ctx, params)
}

// DeleteProjectTokenizedEvents deletes tokenized events
func (s *Service) DeleteProjectTokenizedEvents(ctx context.Context, projectID string, filter *datastore.EventFilter) error {
	return s.repo.HardDeleteTokenizedEvents(ctx, common.StringToPgText(projectID))
}

// CopyRows copies rows from events to events_search
func (s *Service) CopyRows(ctx context.Context, projectID string, interval int) error {
	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// If interval is not default, hard delete tokenized events first
	if interval != config.DefaultSearchTokenizationInterval {
		err = qtx.HardDeleteTokenizedEvents(ctx, common.StringToPgText(projectID))
		if err != nil {
			return err
		}
	}

	// Call PL/pgSQL function to copy rows
	params := repo.CopyRowsFromEventsToEventsSearchParams{
		ProjectID: projectID,
		BatchSize: int32(interval),
	}
	err = qtx.CopyRowsFromEventsToEventsSearch(ctx, params)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ExportRecords exports events to a writer
func (s *Service) ExportRecords(ctx context.Context, projectID string, createdAt time.Time, w io.Writer) (int64, error) {
	// TODO: Implement using pgx instead of sqlx
	// This is a rarely used function for data export
	// For now, return an error indicating it needs implementation
	return 0, errors.New("ExportRecords not yet implemented for pgx - use legacy implementation")
}

// PartitionEventsTable partitions the events table
func (s *Service) PartitionEventsTable(ctx context.Context) error {
	_, err := s.db.Exec(ctx, partitionEventsTableSQL)
	return err
}

// UnPartitionEventsTable un-partitions the events table
func (s *Service) UnPartitionEventsTable(ctx context.Context) error {
	_, err := s.db.Exec(ctx, unPartitionEventsTableSQL)
	return err
}

// PartitionEventsSearchTable partitions the events_search table
func (s *Service) PartitionEventsSearchTable(ctx context.Context) error {
	_, err := s.db.Exec(ctx, partitionEventsSearchTableSQL)
	return err
}

// UnPartitionEventsSearchTable un-partitions the events_search table
func (s *Service) UnPartitionEventsSearchTable(ctx context.Context) error {
	_, err := s.db.Exec(ctx, unPartitionEventsSearchTableSQL)
	return err
}

// Helper: getCreatedDateFilter converts Unix timestamps to time.Time
func getCreatedDateFilter(startDate, endDate int64) (time.Time, time.Time) {
	start := time.Unix(startDate, 0)
	end := time.Unix(endDate, 0)

	// If no start date provided, use beginning of time (Unix epoch)
	if startDate == 0 {
		start = time.Unix(0, 0)
	}

	// If no end date provided, use far future date
	if endDate == 0 {
		// Use year 2100 as a reasonable far future date
		end = time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	return start, end
}

// Partition SQL constants - define and execute PL/pgSQL functions for table partitioning
// These SQL strings create PL/pgSQL functions in the database and then execute them
const partitionEventsTableSQL = `
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
SELECT convoy.partition_events_table();
`

const unPartitionEventsTableSQL = `
CREATE OR REPLACE FUNCTION convoy.un_partition_events_table()
    RETURNS VOID AS $$
BEGIN
    RAISE NOTICE 'Starting un-partitioning of events table...';

    -- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.events_new;

    -- Create non-partitioned table
    CREATE TABLE convoy.events_new
    (
        id                 VARCHAR NOT NULL PRIMARY KEY,
        event_type         TEXT NOT NULL,
        endpoints          TEXT,
        project_id         VARCHAR NOT NULL
            CONSTRAINT events_new_project_id_fkey
                REFERENCES convoy.projects,
        source_id          VARCHAR
            CONSTRAINT events_new_source_id_fkey
                REFERENCES convoy.sources,
        headers            JSONB,
        raw                TEXT NOT NULL,
        data               BYTEA NOT NULL,
        created_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
        updated_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        deleted_at         TIMESTAMP WITH TIME ZONE,
        url_query_params   VARCHAR,
        idempotency_key    TEXT,
        is_duplicate_event BOOLEAN DEFAULT FALSE,
        acknowledged_at    TIMESTAMP WITH TIME ZONE,
        status             TEXT,
        metadata           TEXT
    );

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.events_new SELECT * FROM convoy.events;

    ALTER TABLE convoy.event_deliveries DROP CONSTRAINT IF EXISTS event_deliveries_event_id_fkey;
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
END;
$$ LANGUAGE plpgsql;
SELECT convoy.un_partition_events_table();
`

const partitionEventsSearchTableSQL = `
CREATE OR REPLACE FUNCTION convoy.partition_events_search_table() RETURNS VOID AS $$
DECLARE
    r RECORD;
BEGIN
    RAISE NOTICE 'Creating partitioned table...';

    -- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.events_search_new;

    -- Create partitioned table
    CREATE TABLE convoy.events_search_new (
      id                 VARCHAR NOT NULL,
      event_type         TEXT NOT NULL,
      endpoints          TEXT,
      project_id         VARCHAR NOT NULL REFERENCES convoy.projects,
      source_id          VARCHAR REFERENCES convoy.sources,
      headers            JSONB,
      raw                TEXT NOT NULL,
      data               BYTEA NOT NULL,
      url_query_params   VARCHAR,
      idempotency_key    TEXT,
      is_duplicate_event BOOLEAN DEFAULT FALSE,
      search_token       TSVECTOR GENERATED ALWAYS AS (to_tsvector('simple'::regconfig, raw)) STORED,
      created_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
      updated_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
      deleted_at         TIMESTAMP WITH TIME ZONE,
      PRIMARY KEY (id, created_at, project_id)
    ) PARTITION BY RANGE (project_id, created_at);

    RAISE NOTICE 'Creating partitions...';
    FOR r IN
        WITH dates AS (
            SELECT project_id, created_at::DATE
            FROM convoy.events_search
            GROUP BY created_at::DATE, project_id
        )
        SELECT project_id,
               created_at::TEXT AS start_date,
               (created_at + 1)::TEXT AS stop_date,
               'events_search_' || pg_catalog.REPLACE(project_id::TEXT, '-', '') || '_' || pg_catalog.REPLACE(created_at::TEXT, '-', '') AS partition_table_name
        FROM dates
        LOOP
            EXECUTE FORMAT(
                    'CREATE TABLE IF NOT EXISTS convoy.%s PARTITION OF convoy.events_search_new FOR VALUES FROM (%L, %L) TO (%L, %L)',
                    r.partition_table_name, r.project_id, r.start_date, r.project_id, r.stop_date
                    );
        END LOOP;

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.events_search_new (
        id, event_type, endpoints, project_id, source_id,
        headers, raw, data, url_query_params, idempotency_key,
        is_duplicate_event, created_at, updated_at, deleted_at
    )
    SELECT id, event_type, endpoints, project_id, source_id,
           headers, raw, data, url_query_params, idempotency_key,
           is_duplicate_event, created_at, updated_at, deleted_at
    FROM convoy.events_search;

    -- Manage table renaming
    ALTER TABLE convoy.events_search RENAME TO events_search_old;
    ALTER TABLE convoy.events_search_new RENAME TO events_search;
    DROP TABLE IF EXISTS convoy.events_search_old;

    RAISE NOTICE 'Recreating indexes...';
    CREATE INDEX idx_events_search_id_key ON convoy.events_search (id);
    CREATE INDEX idx_events_search_created_at_key ON convoy.events_search (created_at);
    CREATE INDEX idx_events_search_deleted_at_key ON convoy.events_search (deleted_at);
    CREATE INDEX idx_events_search_project_id_deleted_at_key ON convoy.events_search (project_id, deleted_at);
    CREATE INDEX idx_events_search_project_id_key ON convoy.events_search (project_id);
    CREATE INDEX idx_events_search_project_id_source_id ON convoy.events_search (project_id, source_id);
    CREATE INDEX idx_events_search_source_id ON convoy.events_search (source_id);
    CREATE INDEX idx_events_search_token_key ON convoy.events_search USING gin (search_token);

    RAISE NOTICE 'Migration complete!';
END;
$$ LANGUAGE plpgsql;
SELECT convoy.partition_events_search_table();
`

const unPartitionEventsSearchTableSQL = `
CREATE OR REPLACE FUNCTION convoy.un_partition_events_search_table() RETURNS VOID AS $$
BEGIN
    RAISE NOTICE 'Starting un-partitioning of events_search table...';

    -- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.events_search_new;

    -- Create non-partitioned table
    CREATE TABLE convoy.events_search_new
    (
        id                 VARCHAR NOT NULL PRIMARY KEY,
        event_type         TEXT NOT NULL,
        endpoints          TEXT,
        project_id         VARCHAR NOT NULL REFERENCES convoy.projects,
        source_id          VARCHAR REFERENCES convoy.sources,
        headers            JSONB,
        raw                TEXT NOT NULL,
        data               BYTEA NOT NULL,
        url_query_params   VARCHAR,
        idempotency_key    TEXT,
        is_duplicate_event BOOLEAN DEFAULT FALSE,
        search_token       TSVECTOR GENERATED ALWAYS AS (to_tsvector('simple', raw)) STORED,
        created_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        deleted_at         TIMESTAMP WITH TIME ZONE
    );

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.events_search_new
        (id, event_type, endpoints, project_id,
         source_id, headers, raw, data, url_query_params,
         idempotency_key, is_duplicate_event,
         created_at, updated_at, deleted_at)
    SELECT id, event_type, endpoints, project_id,
           source_id, headers, raw, data, url_query_params,
           idempotency_key, is_duplicate_event,
           created_at, updated_at, deleted_at
    FROM convoy.events_search;

    ALTER TABLE convoy.events_search RENAME TO events_search_old;
    ALTER TABLE convoy.events_search_new RENAME TO events_search;
    DROP TABLE IF EXISTS convoy.events_search_old;

    RAISE NOTICE 'Recreating indexes...';
    CREATE INDEX idx_events_search_id_key ON convoy.events_search (id);
    CREATE INDEX idx_events_search_created_at_key ON convoy.events_search (created_at);
    CREATE INDEX idx_events_search_deleted_at_key ON convoy.events_search (deleted_at);
    CREATE INDEX idx_events_search_project_id_deleted_at_key ON convoy.events_search (project_id, deleted_at);
    CREATE INDEX idx_events_search_project_id_key ON convoy.events_search (project_id);
    CREATE INDEX idx_events_search_project_id_source_id ON convoy.events_search (project_id, source_id);
    CREATE INDEX idx_events_search_source_id ON convoy.events_search (source_id);
    CREATE INDEX idx_events_search_token_key ON convoy.events_search USING gin (search_token);

    RAISE NOTICE 'Successfully un-partitioned events_search table...';
END;
$$ LANGUAGE plpgsql;
SELECT convoy.un_partition_events_search_table();
`
