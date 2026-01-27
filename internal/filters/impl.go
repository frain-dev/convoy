package filters

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/filters/repo"
	"github.com/frain-dev/convoy/pkg/compare"
	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

// Service implements the FilterRepository using SQLc-generated queries
type Service struct {
	logger log.StdLogger
	repo   repo.Querier
	db     *pgxpool.Pool
}

// Ensure Service implements datastore.FilterRepository at compile time
var _ datastore.FilterRepository = (*Service)(nil)

// New creates a new Filter Service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// rowToEventTypeFilter converts SQLc-generated ConvoyFilter to datastore.EventTypeFilter
func rowToEventTypeFilter(row repo.ConvoyFilter) (*datastore.EventTypeFilter, error) {
	headers, err := common.JSONBToM(row.Headers)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal headers: %w", err)
	}

	body, err := common.JSONBToM(row.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal body: %w", err)
	}

	rawHeaders, err := common.JSONBToM(row.RawHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw_headers: %w", err)
	}

	rawBody, err := common.JSONBToM(row.RawBody)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw_body: %w", err)
	}

	return &datastore.EventTypeFilter{
		UID:            row.ID,
		SubscriptionID: row.SubscriptionID,
		EventType:      row.EventType,
		Headers:        headers,
		Body:           body,
		RawHeaders:     rawHeaders,
		RawBody:        rawBody,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}, nil
}

// ============================================================================
// Service Implementation
// ============================================================================

// CreateFilter creates a new filter
func (s *Service) CreateFilter(ctx context.Context, filter *datastore.EventTypeFilter) error {
	if filter == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("filter cannot be nil"))
	}

	// Generate UID if not provided
	if util.IsStringEmpty(filter.UID) {
		filter.UID = ulid.Make().String()
	}

	// Set timestamps if not provided
	if filter.CreatedAt.IsZero() {
		filter.CreatedAt = time.Now()
	}
	if filter.UpdatedAt.IsZero() {
		filter.UpdatedAt = time.Now()
	}

	// Flatten body and headers for matching
	flatBody, err := common.FlattenM(filter.Body)
	if err != nil {
		s.logger.WithError(err).Error("failed to flatten body filter")
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to flatten body filter: %w", err))
	}

	flatHeaders, err := common.FlattenM(filter.Headers)
	if err != nil {
		s.logger.WithError(err).Error("failed to flatten header filter")
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to flatten header filter: %w", err))
	}

	// Convert to JSONB
	headersJSON, err := common.MToJSONB(flatHeaders)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal headers: %w", err))
	}

	bodyJSON, err := common.MToJSONB(flatBody)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal body: %w", err))
	}

	rawHeadersJSON, err := common.MToJSONB(filter.RawHeaders)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal raw_headers: %w", err))
	}

	rawBodyJSON, err := common.MToJSONB(filter.RawBody)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal raw_body: %w", err))
	}

	// Update the filter with flattened values
	filter.Headers = flatHeaders
	filter.Body = flatBody

	// Create filter
	err = s.repo.CreateFilter(ctx, repo.CreateFilterParams{
		ID:             filter.UID,
		SubscriptionID: filter.SubscriptionID,
		EventType:      filter.EventType,
		Headers:        headersJSON,
		Body:           bodyJSON,
		RawHeaders:     rawHeadersJSON,
		RawBody:        rawBodyJSON,
		CreatedAt:      pgtype.Timestamptz{Time: filter.CreatedAt, Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: filter.UpdatedAt, Valid: true},
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to create filter")
		return util.NewServiceError(http.StatusInternalServerError, errors.New("filter could not be created"))
	}

	return nil
}

// CreateFilters creates multiple filters in a transaction
func (s *Service) CreateFilters(ctx context.Context, filters []datastore.EventTypeFilter) error {
	if len(filters) == 0 {
		return nil // No-op for empty array
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Create each filter in the transaction
	for i := range filters {
		filter := &filters[i]

		// Generate UID if not provided
		if util.IsStringEmpty(filter.UID) {
			filter.UID = ulid.Make().String()
		}

		// Set timestamps if not provided
		if filter.CreatedAt.IsZero() {
			filter.CreatedAt = time.Now()
		}
		if filter.UpdatedAt.IsZero() {
			filter.UpdatedAt = time.Now()
		}

		// Flatten body and headers for matching
		flatBody, err := common.FlattenM(filter.Body)
		if err != nil {
			s.logger.WithError(err).Error("failed to flatten body filter")
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to flatten body filter: %w", err))
		}

		flatHeaders, err := common.FlattenM(filter.Headers)
		if err != nil {
			s.logger.WithError(err).Error("failed to flatten header filter")
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to flatten header filter: %w", err))
		}

		// Convert to JSONB
		headersJSON, err := common.MToJSONB(flatHeaders)
		if err != nil {
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal headers: %w", err))
		}

		bodyJSON, err := common.MToJSONB(flatBody)
		if err != nil {
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal body: %w", err))
		}

		rawHeadersJSON, err := common.MToJSONB(filter.RawHeaders)
		if err != nil {
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal raw_headers: %w", err))
		}

		rawBodyJSON, err := common.MToJSONB(filter.RawBody)
		if err != nil {
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal raw_body: %w", err))
		}

		// Update the filter with flattened values
		filter.Headers = flatHeaders
		filter.Body = flatBody

		// Create filter
		err = qtx.CreateFilter(ctx, repo.CreateFilterParams{
			ID:             filter.UID,
			SubscriptionID: filter.SubscriptionID,
			EventType:      filter.EventType,
			Headers:        headersJSON,
			Body:           bodyJSON,
			RawHeaders:     rawHeadersJSON,
			RawBody:        rawBodyJSON,
			CreatedAt:      pgtype.Timestamptz{Time: filter.CreatedAt, Valid: true},
			UpdatedAt:      pgtype.Timestamptz{Time: filter.UpdatedAt, Valid: true},
		})

		if err != nil {
			s.logger.WithError(err).Error("failed to create filter")
			return util.NewServiceError(http.StatusInternalServerError, errors.New("filter could not be created"))
		}
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// UpdateFilter updates an existing filter
func (s *Service) UpdateFilter(ctx context.Context, filter *datastore.EventTypeFilter) error {
	if filter == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("filter cannot be nil"))
	}

	// Flatten body and headers for matching
	flatBody, err := common.FlattenM(filter.Body)
	if err != nil {
		s.logger.WithError(err).Error("failed to flatten body filter")
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to flatten body filter: %w", err))
	}

	flatHeaders, err := common.FlattenM(filter.Headers)
	if err != nil {
		s.logger.WithError(err).Error("failed to flatten header filter")
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to flatten header filter: %w", err))
	}

	// Convert to JSONB
	headersJSON, err := common.MToJSONB(flatHeaders)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal headers: %w", err))
	}

	bodyJSON, err := common.MToJSONB(flatBody)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal body: %w", err))
	}

	rawHeadersJSON, err := common.MToJSONB(filter.RawHeaders)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal raw_headers: %w", err))
	}

	rawBodyJSON, err := common.MToJSONB(filter.RawBody)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal raw_body: %w", err))
	}

	// Update the filter with flattened values
	filter.Headers = flatHeaders
	filter.Body = flatBody

	// Update filter
	rowsAffected, err := s.repo.UpdateFilter(ctx, repo.UpdateFilterParams{
		ID:         filter.UID,
		Headers:    headersJSON,
		Body:       bodyJSON,
		RawHeaders: rawHeadersJSON,
		RawBody:    rawBodyJSON,
		EventType:  filter.EventType,
		UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to update filter")
		return util.NewServiceError(http.StatusInternalServerError, errors.New("filter could not be updated"))
	}

	if rowsAffected == 0 {
		return datastore.ErrFilterNotFound
	}

	return nil
}

// UpdateFilters updates multiple filters in a transaction
func (s *Service) UpdateFilters(ctx context.Context, filters []datastore.EventTypeFilter) error {
	if len(filters) == 0 {
		return nil // No-op for empty array
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Update each filter in the transaction
	for i := range filters {
		filter := &filters[i]

		// Flatten body and headers for matching
		flatBody, err := common.FlattenM(filter.Body)
		if err != nil {
			s.logger.WithError(err).Error("failed to flatten body filter")
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to flatten body filter: %w", err))
		}

		flatHeaders, err := common.FlattenM(filter.Headers)
		if err != nil {
			s.logger.WithError(err).Error("failed to flatten header filter")
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to flatten header filter: %w", err))
		}

		// Convert to JSONB
		headersJSON, err := common.MToJSONB(flatHeaders)
		if err != nil {
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal headers: %w", err))
		}

		bodyJSON, err := common.MToJSONB(flatBody)
		if err != nil {
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal body: %w", err))
		}

		rawHeadersJSON, err := common.MToJSONB(filter.RawHeaders)
		if err != nil {
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal raw_headers: %w", err))
		}

		rawBodyJSON, err := common.MToJSONB(filter.RawBody)
		if err != nil {
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to marshal raw_body: %w", err))
		}

		// Update the filter with flattened values
		filter.Headers = flatHeaders
		filter.Body = flatBody
		filter.UpdatedAt = time.Now()

		// Update filter
		rowsAffected, err := qtx.UpdateFilter(ctx, repo.UpdateFilterParams{
			ID:         filter.UID,
			Headers:    headersJSON,
			Body:       bodyJSON,
			RawHeaders: rawHeadersJSON,
			RawBody:    rawBodyJSON,
			EventType:  filter.EventType,
			UpdatedAt:  pgtype.Timestamptz{Time: filter.UpdatedAt, Valid: true},
		})

		if err != nil {
			s.logger.WithError(err).Errorf("failed to update filter %s", filter.UID)
			return util.NewServiceError(http.StatusInternalServerError, fmt.Errorf("failed to update filter %s", filter.UID))
		}

		if rowsAffected == 0 {
			return util.NewServiceError(http.StatusNotFound, fmt.Errorf("filter %s not found", filter.UID))
		}
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// DeleteFilter deletes a filter by ID
func (s *Service) DeleteFilter(ctx context.Context, filterID string) error {
	rowsAffected, err := s.repo.DeleteFilter(ctx, filterID)
	if err != nil {
		s.logger.WithError(err).Error("failed to delete filter")
		return util.NewServiceError(http.StatusInternalServerError, errors.New("filter could not be deleted"))
	}

	if rowsAffected == 0 {
		return datastore.ErrFilterNotFound
	}

	return nil
}

// FindFilterByID retrieves a filter by its ID
func (s *Service) FindFilterByID(ctx context.Context, filterID string) (*datastore.EventTypeFilter, error) {
	row, err := s.repo.FindFilterByID(ctx, filterID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrFilterNotFound
		}
		s.logger.WithError(err).Error("failed to find filter by id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	filter, err := rowToEventTypeFilter(row)
	if err != nil {
		s.logger.WithError(err).Error("failed to convert row to filter")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return filter, nil
}

// FindFiltersBySubscriptionID retrieves all filters for a subscription
func (s *Service) FindFiltersBySubscriptionID(ctx context.Context, subscriptionID string) ([]datastore.EventTypeFilter, error) {
	rows, err := s.repo.FindFiltersBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		s.logger.WithError(err).Error("failed to find filters by subscription id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	filters := make([]datastore.EventTypeFilter, 0, len(rows))
	for _, row := range rows {
		filter, err := rowToEventTypeFilter(row)
		if err != nil {
			s.logger.WithError(err).Error("failed to convert row to filter")
			return nil, util.NewServiceError(http.StatusInternalServerError, err)
		}
		filters = append(filters, *filter)
	}

	return filters, nil
}

// FindFilterBySubscriptionAndEventType retrieves a filter by subscription and event type
func (s *Service) FindFilterBySubscriptionAndEventType(ctx context.Context, subscriptionID, eventType string) (*datastore.EventTypeFilter, error) {
	row, err := s.repo.FindFilterBySubscriptionAndEventType(ctx, repo.FindFilterBySubscriptionAndEventTypeParams{
		SubscriptionID: subscriptionID,
		EventType:      eventType,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrFilterNotFound
		}
		s.logger.WithError(err).Error("failed to find filter by subscription and event type")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	filter, err := rowToEventTypeFilter(row)
	if err != nil {
		s.logger.WithError(err).Error("failed to convert row to filter")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return filter, nil
}

// TestFilter tests if a payload matches a filter
func (s *Service) TestFilter(ctx context.Context, subscriptionID, eventType string, payload any) (bool, error) {
	// Try to find the filter for the specific event type
	filter, err := s.FindFilterBySubscriptionAndEventType(ctx, subscriptionID, eventType)
	if err != nil {
		if errors.Is(err, datastore.ErrFilterNotFound) {
			// If no filter exists for this event type, check for a catch-all filter
			filter, err = s.FindFilterBySubscriptionAndEventType(ctx, subscriptionID, "*")
			if err != nil {
				if errors.Is(err, datastore.ErrFilterNotFound) {
					// No filtering, so it matches
					return true, nil
				}
				return false, err
			}
		} else {
			return false, err
		}
	}

	// Empty filter means it matches everything
	if len(filter.Body) == 0 {
		return true, nil
	}

	// Flatten the payload for comparison
	p, err := flatten.Flatten(payload)
	if err != nil {
		return false, err
	}

	// Compare flattened payload against filter body
	return compare.Compare(p, filter.Body)
}
