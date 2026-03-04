package events

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/jackc/pgx/v5"

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
	db     database.Database
}

// Ensure Service implements datastore.EventRepository at compile time
var _ datastore.EventRepository = (*Service)(nil)

// New creates a new events service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db,
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
	tx, err := s.db.GetConn().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.WithTx(tx)

	// Create event params
	params := repo.CreateEventParams{
		ID:               event.UID,
		EventType:        string(event.EventType),
		Endpoints:        endpointsToString(event.Endpoints),
		ProjectID:        event.ProjectID,
		SourceID:         common.StringPtrToPgText(sourceID),
		Headers:          headersToJSONB(event.Headers),
		Raw:              event.Raw,
		Data:             event.Data,
		URLQueryParams:   event.URLQueryParams,
		IdempotencyKey:   event.IdempotencyKey,
		IsDuplicateEvent: event.IsDuplicateEvent,
		AcknowledgedAt:   common.NullTimeToPgTimestamptz(event.AcknowledgedAt),
		Metadata:         common.StringToPgText(event.Metadata),
		Status:           string(event.Status),
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
			err = qtx.CreateEventEndpoints(ctx, event.UID, endpointID)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// FindEventByID finds an event by ID
func (s *Service) FindEventByID(ctx context.Context, projectID, id string) (*datastore.Event, error) {
	row, err := s.repo.FindEventByID(ctx, id, projectID)
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
	rows, err := s.repo.FindEventsByIDs(ctx, ids, projectID)
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
	rows, err := s.repo.FindEventsByIdempotencyKey(ctx, idempotencyKey, projectID)
	if err != nil {
		return nil, err
	}

	events := make([]datastore.Event, 0, len(rows))
	for _, row := range rows {
		// These rows only have ID, need to fetch full event
		event, err := s.FindEventByID(ctx, projectID, row.ID)
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}

	return events, nil
}

// FindFirstEventWithIdempotencyKey finds the first non-duplicate event
func (s *Service) FindFirstEventWithIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (*datastore.Event, error) {
	row, err := s.repo.FindFirstEventWithIdempotencyKey(ctx, idempotencyKey, projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrEventNotFound
		}
		return nil, err
	}

	// Fetch full event details
	return s.FindEventByID(ctx, projectID, row.ID)
}

// UpdateEventEndpoints updates event endpoints with batch processing
func (s *Service) UpdateEventEndpoints(ctx context.Context, event *datastore.Event, endpoints []string) error {
	// Start transaction
	tx, err := s.db.GetConn().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.WithTx(tx)

	// Update endpoints array
	err = qtx.UpdateEventEndpoints(ctx, endpointsToString(event.Endpoints), event.ProjectID, event.UID)
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
			err = qtx.CreateEventEndpoints(ctx, event.UID, endpointID)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// UpdateEventStatus updates event status
func (s *Service) UpdateEventStatus(ctx context.Context, event *datastore.Event, status datastore.EventStatus) error {
	return s.repo.UpdateEventStatus(ctx, string(status), event.ProjectID, event.UID)
}

// CountProjectMessages counts total events in a project
func (s *Service) CountProjectMessages(ctx context.Context, projectID string) (int64, error) {
	return s.repo.CountProjectMessages(ctx, projectID)
}

// CountEvents counts events with filters
func (s *Service) CountEvents(ctx context.Context, projectID string, filter *datastore.Filter) (int64, error) {
	startDate, endDate := getCreatedDateFilter(filter.SearchParams.CreatedAtStart, filter.SearchParams.CreatedAtEnd)

	params := repo.CountEventsParams{
		ProjectID:      projectID,
		StartDate:      startDate,
		EndDate:        endDate,
		HasEndpointIds: len(filter.EndpointIDs) > 0,
		EndpointIds:    filter.EndpointIDs,
		HasSourceID:    !util.IsStringEmpty(filter.SourceID),
		SourceID:       filter.SourceID,
	}

	return s.repo.CountEvents(ctx, params)
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
		HasEndpointOrOwnerFilter: !util.IsStringEmpty(filter.OwnerID) || len(filter.EndpointIDs) > 0,
		HasOwnerID:               !util.IsStringEmpty(filter.OwnerID),
		OwnerID:                  filter.OwnerID,
		HasEndpointIds:           len(filter.EndpointIDs) > 0,
		EndpointIds:              filter.EndpointIDs,
		ProjectID:                projectID,
		HasIdempotencyKey:        !util.IsStringEmpty(filter.IdempotencyKey),
		IdempotencyKey:           filter.IdempotencyKey,
		StartDate:                startDate,
		EndDate:                  endDate,
		HasSourceIds:             len(filter.SourceIDs) > 0,
		SourceIds:                filter.SourceIDs,
		HasBrokerMessageID:       !util.IsStringEmpty(filter.BrokerMessageId),
		BrokerMessageID:          filter.BrokerMessageId,
		HasCursor:                hasCursor && cursorLte,
		Cursor:                   cursor,
		CursorGte:                hasCursor && cursorGte,
		SortAsc:                  sortAsc,
		Limit:                    int32(filter.Pageable.Limit()),
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
		ProjectID:          projectID,
		HasIdempotencyKey:  !util.IsStringEmpty(filter.IdempotencyKey),
		IdempotencyKey:     filter.IdempotencyKey,
		StartDate:          startDate,
		EndDate:            endDate,
		HasSourceIds:       len(filter.SourceIDs) > 0,
		SourceIds:          filter.SourceIDs,
		HasEndpointIds:     len(filter.EndpointIDs) > 0,
		EndpointIds:        filter.EndpointIDs,
		HasBrokerMessageID: !util.IsStringEmpty(filter.BrokerMessageId),
		BrokerMessageID:    filter.BrokerMessageId,
		HasQuery:           !util.IsStringEmpty(filter.Query),
		Query:              filter.Query,
		HasCursor:          hasCursor && cursorLte,
		Cursor:             cursor,
		CursorGte:          hasCursor && cursorGte,
		SortAsc:            sortAsc,
		Limit:              int32(filter.Pageable.Limit()),
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
			ProjectID:          projectID,
			HasIdempotencyKey:  !util.IsStringEmpty(filter.IdempotencyKey),
			IdempotencyKey:     filter.IdempotencyKey,
			StartDate:          startDate,
			EndDate:            endDate,
			HasSourceIds:       len(filter.SourceIDs) > 0,
			SourceIds:          filter.SourceIDs,
			HasEndpointIds:     len(filter.EndpointIDs) > 0,
			EndpointIds:        filter.EndpointIDs,
			HasBrokerMessageID: !util.IsStringEmpty(filter.BrokerMessageId),
			BrokerMessageID:    filter.BrokerMessageId,
			SortAsc:            sortAsc,
			Cursor:             cursor,
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
		ProjectID:          projectID,
		HasIdempotencyKey:  !util.IsStringEmpty(filter.IdempotencyKey),
		IdempotencyKey:     filter.IdempotencyKey,
		StartDate:          startDate,
		EndDate:            endDate,
		HasSourceIds:       len(filter.SourceIDs) > 0,
		SourceIds:          filter.SourceIDs,
		HasEndpointIds:     len(filter.EndpointIDs) > 0,
		EndpointIds:        filter.EndpointIDs,
		HasBrokerMessageID: !util.IsStringEmpty(filter.BrokerMessageId),
		BrokerMessageID:    filter.BrokerMessageId,
		HasQuery:           !util.IsStringEmpty(filter.Query),
		Query:              filter.Query,
		SortAsc:            sortAsc,
		Cursor:             cursor,
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
		return s.repo.HardDeleteProjectEvents(ctx, projectID, startDate, endDate)
	}

	return s.repo.SoftDeleteProjectEvents(ctx, projectID, startDate, endDate)
}

// DeleteProjectTokenizedEvents deletes tokenized events
func (s *Service) DeleteProjectTokenizedEvents(ctx context.Context, projectID string, filter *datastore.EventFilter) error {
	return s.repo.HardDeleteTokenizedEvents(ctx, projectID)
}

// CopyRows copies rows from events to events_search
func (s *Service) CopyRows(ctx context.Context, projectID string, interval int) error {
	// Start transaction
	tx, err := s.db.GetConn().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.WithTx(tx)

	// If interval is not default, hard delete tokenized events first
	if interval != config.DefaultSearchTokenizationInterval {
		err = qtx.HardDeleteTokenizedEvents(ctx, projectID)
		if err != nil {
			return err
		}
	}

	// Call PL/pgSQL function to copy rows
	err = qtx.CopyRowsFromEventsToEventsSearch(ctx, projectID, int32(interval))
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
	return s.repo.PartitionEventsTable(ctx)
}

// UnPartitionEventsTable un-partitions the events table
func (s *Service) UnPartitionEventsTable(ctx context.Context) error {
	return s.repo.UnPartitionEventsTable(ctx)
}

// PartitionEventsSearchTable partitions the events_search table
func (s *Service) PartitionEventsSearchTable(ctx context.Context) error {
	return s.repo.PartitionEventsSearchTable(ctx)
}

// UnPartitionEventsSearchTable un-partitions the events_search table
func (s *Service) UnPartitionEventsSearchTable(ctx context.Context) error {
	return s.repo.UnPartitionEventsSearchTable(ctx)
}

// Helper: getCreatedDateFilter converts Unix timestamps to time.Time
func getCreatedDateFilter(startDate, endDate int64) (time.Time, time.Time) {
	return time.Unix(startDate, 0), time.Unix(endDate, 0)
}
