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
		UrlQueryParams:   common.StringToPgText(event.URLQueryParams),
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
		Endpoints: common.StringToPgText(endpointsToString(event.Endpoints)),
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

	// Trim LIMIT+1 for hasNext detection
	if len(events) > filter.Pageable.PerPage {
		events = events[:len(events)-1]
		ids = ids[:len(ids)-1]
	}

	// Build pagination metadata
	pagination := &datastore.PaginationData{PrevRowCount: rowCount}
	pagination = pagination.Build(filter.Pageable, ids)

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
	// Not implemented - partition functions are commented out in queries.sql
	return errors.New("PartitionEventsTable not implemented - partition functions commented out in queries.sql")
}

// UnPartitionEventsTable un-partitions the events table
func (s *Service) UnPartitionEventsTable(ctx context.Context) error {
	// Not implemented - partition functions are commented out in queries.sql
	return errors.New("UnPartitionEventsTable not implemented - partition functions commented out in queries.sql")
}

// PartitionEventsSearchTable partitions the events_search table
func (s *Service) PartitionEventsSearchTable(ctx context.Context) error {
	// Not implemented - partition functions are commented out in queries.sql
	return errors.New("PartitionEventsSearchTable not implemented - partition functions commented out in queries.sql")
}

// UnPartitionEventsSearchTable un-partitions the events_search table
func (s *Service) UnPartitionEventsSearchTable(ctx context.Context) error {
	// Not implemented - partition functions are commented out in queries.sql
	return errors.New("UnPartitionEventsSearchTable not implemented - partition functions commented out in queries.sql")
}

// Helper: getCreatedDateFilter converts Unix timestamps to time.Time
func getCreatedDateFilter(startDate, endDate int64) (time.Time, time.Time) {
	return time.Unix(startDate, 0), time.Unix(endDate, 0)
}
