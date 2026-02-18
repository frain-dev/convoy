package meta_events

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/meta_events/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrMetaEventNotCreated = errors.New("meta event could not be created")
	ErrMetaEventNotUpdated = errors.New("meta event could not be updated")
)

// Service implements the meta event Service using SQLc-generated queries
type Service struct {
	logger log.StdLogger
	repo   repo.Querier  // SQLc-generated interface
	db     *pgxpool.Pool // Connection pool
}

// Ensure Service implements datastore.MetaEventRepository at compile time
var _ datastore.MetaEventRepository = (*Service)(nil)

// New creates a new meta event Service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// rowToMetaEvent converts any SQLc-generated row struct to datastore.MetaEvent
func (s *Service) rowToMetaEvent(row interface{}) (datastore.MetaEvent, error) {
	var (
		id, projectID, eventType, status string
		metadata                         []byte
		attempt                          []byte
		createdAt, updatedAt             pgtype.Timestamptz
	)

	switch r := row.(type) {
	case repo.FindMetaEventByIDRow:
		id, projectID, eventType, status = r.ID, r.ProjectID, r.EventType, r.Status
		metadata, attempt = r.Metadata, r.Attempt
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
	case repo.FetchMetaEventsPaginatedRow:
		id, projectID, eventType, status = r.ID, r.ProjectID, r.EventType, r.Status
		metadata, attempt = r.Metadata, r.Attempt
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
	default:
		return datastore.MetaEvent{}, errors.New("unknown row type")
	}

	// Parse metadata JSONB
	var metadataPtr *datastore.Metadata
	if len(metadata) > 0 && string(metadata) != "null" {
		var m datastore.Metadata
		if err := json.Unmarshal(metadata, &m); err != nil {
			s.logger.WithError(err).Error("failed to unmarshal metadata")
			return datastore.MetaEvent{}, err
		}
		metadataPtr = &m
	}

	// Parse attempt JSONB (nullable)
	var attemptPtr *datastore.MetaEventAttempt
	if len(attempt) > 0 && string(attempt) != "null" {
		var a datastore.MetaEventAttempt
		if err := json.Unmarshal(attempt, &a); err != nil {
			s.logger.WithError(err).Error("failed to unmarshal attempt")
			return datastore.MetaEvent{}, err
		}
		attemptPtr = &a
	}

	return datastore.MetaEvent{
		UID:       id,
		ProjectID: projectID,
		EventType: eventType,
		Metadata:  metadataPtr,
		Attempt:   attemptPtr,
		Status:    datastore.EventDeliveryStatus(status),
		CreatedAt: createdAt.Time,
		UpdatedAt: updatedAt.Time,
	}, nil
}

// metadataToJSONB converts *datastore.Metadata to JSONB []byte
func metadataToJSONB(m *datastore.Metadata) ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// attemptToJSONB converts *datastore.MetaEventAttempt to JSONB []byte
func attemptToJSONB(a *datastore.MetaEventAttempt) ([]byte, error) {
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}

// ============================================================================
// Service Implementation
// ============================================================================

// CreateMetaEvent creates a new meta event
func (s *Service) CreateMetaEvent(ctx context.Context, metaEvent *datastore.MetaEvent) error {
	if metaEvent == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("meta event cannot be nil"))
	}

	// Convert metadata to JSONB
	metadataBytes, err := metadataToJSONB(metaEvent.Metadata)
	if err != nil {
		s.logger.WithError(err).Error("failed to marshal metadata")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	err = s.repo.CreateMetaEvent(ctx, repo.CreateMetaEventParams{
		ID:        metaEvent.UID,
		EventType: metaEvent.EventType,
		ProjectID: metaEvent.ProjectID,
		Metadata:  metadataBytes,
		Status:    string(metaEvent.Status),
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to create meta event")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// FindMetaEventByID retrieves a meta event by its ID
func (s *Service) FindMetaEventByID(ctx context.Context, projectID, id string) (*datastore.MetaEvent, error) {
	row, err := s.repo.FindMetaEventByID(ctx, repo.FindMetaEventByIDParams{
		ID:        id,
		ProjectID: projectID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrMetaEventNotFound
		}
		s.logger.WithError(err).Error("failed to find meta event by id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	metaEvent, err := s.rowToMetaEvent(row)
	if err != nil {
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}
	return &metaEvent, nil
}

// LoadMetaEventsPaged retrieves meta events with pagination and filtering
func (s *Service) LoadMetaEventsPaged(ctx context.Context, projectID string, filter *datastore.Filter) ([]datastore.MetaEvent, datastore.PaginationData, error) {
	// Determine direction for SQL query
	direction := "next"
	if filter.Pageable.Direction == datastore.Prev {
		direction = "prev"
	}

	// Get date filters
	startDate, endDate := getCreatedDateFilter(filter.SearchParams.CreatedAtStart, filter.SearchParams.CreatedAtEnd)

	// Convert cursor to pgtype.Text (empty string is handled in SQL query)
	cursor := pgtype.Text{String: filter.Pageable.Cursor(), Valid: true}

	// Query with unified pagination
	rows, err := s.repo.FetchMetaEventsPaginated(ctx, repo.FetchMetaEventsPaginatedParams{
		Direction: direction,
		ProjectID: projectID,
		StartDate: pgtype.Timestamptz{Time: startDate, Valid: true},
		EndDate:   pgtype.Timestamptz{Time: endDate, Valid: true},
		Cursor:    cursor,
		LimitVal:  int64(filter.Pageable.Limit()),
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to load meta events paged")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Convert rows to domain objects
	metaEvents := make([]datastore.MetaEvent, 0, len(rows))
	for _, row := range rows {
		metaEvent, err := s.rowToMetaEvent(row)
		if err != nil {
			return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err)
		}
		metaEvents = append(metaEvents, metaEvent)
	}

	// Detect hasNext by checking if we got more than requested
	hasNext := false
	if len(metaEvents) > filter.Pageable.PerPage {
		hasNext = true
		metaEvents = metaEvents[:filter.Pageable.PerPage] // Trim to actual page size
	}

	// Get first and last items for cursor values
	var first, last datastore.MetaEvent
	if len(metaEvents) > 0 {
		first = metaEvents[0]
		last = metaEvents[len(metaEvents)-1]
	}

	// Count previous rows for pagination metadata
	var prevRowCount datastore.PrevRowCount
	if len(metaEvents) > 0 {
		count, err2 := s.repo.CountPrevMetaEvents(ctx, repo.CountPrevMetaEventsParams{
			ProjectID: projectID,
			StartDate: pgtype.Timestamptz{Time: startDate, Valid: true},
			EndDate:   pgtype.Timestamptz{Time: endDate, Valid: true},
			Cursor:    first.UID,
		})
		if err2 != nil {
			s.logger.WithError(err2).Error("failed to count prev meta events")
			return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err2)
		}
		prevRowCount.Count = int(count.Int64)
	}

	// Build pagination metadata
	ids := make([]string, len(metaEvents))
	for i := range metaEvents {
		ids[i] = metaEvents[i].UID
	}

	pagination := &datastore.PaginationData{
		PrevRowCount:    prevRowCount,
		HasNextPage:     hasNext,
		HasPreviousPage: prevRowCount.Count > 0,
	}

	if len(ids) > 0 {
		pagination.PrevPageCursor = first.UID
		pagination.NextPageCursor = last.UID
	}

	pagination = pagination.Build(filter.Pageable, ids)

	return metaEvents, *pagination, nil
}

// UpdateMetaEvent updates an existing meta event
func (s *Service) UpdateMetaEvent(ctx context.Context, projectID string, metaEvent *datastore.MetaEvent) error {
	if metaEvent == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("meta event cannot be nil"))
	}

	// Convert metadata to JSONB
	metadataBytes, err := metadataToJSONB(metaEvent.Metadata)
	if err != nil {
		s.logger.WithError(err).Error("failed to marshal metadata")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Convert attempt to JSONB
	attemptBytes, err := attemptToJSONB(metaEvent.Attempt)
	if err != nil {
		s.logger.WithError(err).Error("failed to marshal attempt")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	result, err := s.repo.UpdateMetaEvent(ctx, repo.UpdateMetaEventParams{
		ID:        metaEvent.UID,
		ProjectID: projectID,
		EventType: metaEvent.EventType,
		Metadata:  metadataBytes,
		Attempt:   attemptBytes,
		Status:    string(metaEvent.Status),
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to update meta event")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if result.RowsAffected() < 1 {
		return ErrMetaEventNotUpdated
	}

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// getCreatedDateFilter converts Unix timestamps to time.Time
func getCreatedDateFilter(startDate, endDate int64) (time.Time, time.Time) {
	return time.Unix(startDate, 0), time.Unix(endDate, 0)
}
