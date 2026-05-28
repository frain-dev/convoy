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
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

// Service implements the FilterRepository using SQLc-generated queries
type Service struct {
	logger log.Logger
	repo   repo.Querier
	db     *pgxpool.Pool
}

// Ensure Service implements datastore.FilterRepository at compile time
var _ datastore.FilterRepository = (*Service)(nil)

// New creates a new Filter Service
func New(logger log.Logger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// rowToEventTypeFilter converts SQLc-generated row types to datastore.EventTypeFilter
func rowToEventTypeFilter(row interface{}) (*datastore.EventTypeFilter, error) {
	var (
		id, subscriptionID, eventType                                      string
		headers, body, query, path, rawHeaders, rawBody, rawQuery, rawPath []byte
		enabledAt, createdAt, updatedAt                                    pgtype.Timestamptz
	)

	switch r := row.(type) {
	case repo.FindFilterByIDRow:
		id, subscriptionID, eventType = r.ID, r.SubscriptionID, r.EventType
		enabledAt = r.EnabledAt
		headers, body, query, path = r.Headers, r.Body, r.Query, r.Path
		rawHeaders, rawBody, rawQuery, rawPath = r.RawHeaders, r.RawBody, r.RawQuery, r.RawPath
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
	case repo.FindFiltersBySubscriptionIDRow:
		id, subscriptionID, eventType = r.ID, r.SubscriptionID, r.EventType
		enabledAt = r.EnabledAt
		headers, body, query, path = r.Headers, r.Body, r.Query, r.Path
		rawHeaders, rawBody, rawQuery, rawPath = r.RawHeaders, r.RawBody, r.RawQuery, r.RawPath
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
	case repo.FindFilterBySubscriptionAndEventTypeRow:
		id, subscriptionID, eventType = r.ID, r.SubscriptionID, r.EventType
		enabledAt = r.EnabledAt
		headers, body, query, path = r.Headers, r.Body, r.Query, r.Path
		rawHeaders, rawBody, rawQuery, rawPath = r.RawHeaders, r.RawBody, r.RawQuery, r.RawPath
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
	default:
		return nil, fmt.Errorf("unsupported row type: %T", row)
	}

	headersMap, err := common.JSONBToM(headers)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal headers: %w", err)
	}

	bodyMap, err := common.JSONBToM(body)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal body: %w", err)
	}

	queryMap, err := common.JSONBToM(query)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal query: %w", err)
	}

	pathMap, err := common.JSONBToM(path)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal path: %w", err)
	}

	rawHeadersMap, err := common.JSONBToM(rawHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw_headers: %w", err)
	}

	rawBodyMap, err := common.JSONBToM(rawBody)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw_body: %w", err)
	}

	rawQueryMap, err := common.JSONBToM(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw_query: %w", err)
	}

	rawPathMap, err := common.JSONBToM(rawPath)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw_path: %w", err)
	}

	var enabledAtTime *time.Time
	if enabledAt.Valid {
		enabledAtTime = &enabledAt.Time
	}

	return &datastore.EventTypeFilter{
		UID:            id,
		SubscriptionID: subscriptionID,
		EventType:      eventType,
		EnabledAt:      enabledAtTime,
		EnabledAtSet:   true,
		Headers:        headersMap,
		Body:           bodyMap,
		Query:          queryMap,
		Path:           pathMap,
		RawHeaders:     rawHeadersMap,
		RawBody:        rawBodyMap,
		RawQuery:       rawQueryMap,
		RawPath:        rawPathMap,
		CreatedAt:      createdAt.Time,
		UpdatedAt:      updatedAt.Time,
	}, nil
}

type preparedFilterMaps struct {
	headers, body, query, path             []byte
	rawHeaders, rawBody, rawQuery, rawPath []byte
}

func prepareFilterMaps(filter *datastore.EventTypeFilter) (*preparedFilterMaps, error) {
	flatBody, err := common.FlattenM(filter.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to flatten body filter: %w", err)
	}

	flatHeaders, err := common.FlattenM(filter.Headers)
	if err != nil {
		return nil, fmt.Errorf("failed to flatten header filter: %w", err)
	}

	flatQuery, err := common.FlattenM(filter.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to flatten query filter: %w", err)
	}

	flatPath, err := common.FlattenM(filter.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to flatten path filter: %w", err)
	}

	if err := validateNonBodyFilterScope("header", flatHeaders); err != nil {
		return nil, err
	}
	if err := validateNonBodyFilterScope("query", flatQuery); err != nil {
		return nil, err
	}
	if err := validateNonBodyFilterScope("path", flatPath); err != nil {
		return nil, err
	}

	headersJSON, err := common.MToJSONB(flatHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal headers: %w", err)
	}

	bodyJSON, err := common.MToJSONB(flatBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	queryJSON, err := common.MToJSONB(flatQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	pathJSON, err := common.MToJSONB(flatPath)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal path: %w", err)
	}

	rawHeadersJSON, err := common.MToJSONB(filter.RawHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal raw_headers: %w", err)
	}

	rawBodyJSON, err := common.MToJSONB(filter.RawBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal raw_body: %w", err)
	}

	rawQueryJSON, err := common.MToJSONB(filter.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal raw_query: %w", err)
	}

	rawPathJSON, err := common.MToJSONB(filter.RawPath)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal raw_path: %w", err)
	}

	filter.Headers = flatHeaders
	filter.Body = flatBody
	filter.Query = flatQuery
	filter.Path = flatPath

	return &preparedFilterMaps{
		headers: headersJSON, body: bodyJSON, query: queryJSON, path: pathJSON,
		rawHeaders: rawHeadersJSON, rawBody: rawBodyJSON, rawQuery: rawQueryJSON, rawPath: rawPathJSON,
	}, nil
}

func validateNonBodyFilterScope(scope string, filter datastore.M) error {
	if datastore.HasArrayWildcardSelector(filter) {
		return fmt.Errorf("array wildcard selectors are unsupported for %s filters", scope)
	}

	return nil
}

func setDefaultEnabledAt(filter *datastore.EventTypeFilter) {
	if filter.EnabledAt != nil || filter.EnabledAtSet {
		return
	}

	now := time.Now()
	filter.EnabledAt = &now
	filter.EnabledAtSet = true
}

func timeToPgTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}

	return pgtype.Timestamptz{Time: *t, Valid: true}
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
	setDefaultEnabledAt(filter)

	maps, err := prepareFilterMaps(filter)
	if err != nil {
		s.logger.Error("failed to prepare filter", "error", err)
		return util.NewServiceError(http.StatusBadRequest, err)
	}

	// Create filter
	err = s.repo.CreateFilter(ctx, repo.CreateFilterParams{
		ID:             common.StringToPgText(filter.UID),
		SubscriptionID: common.StringToPgText(filter.SubscriptionID),
		EventType:      common.StringToPgText(filter.EventType),
		EnabledAt:      timeToPgTimestamptz(filter.EnabledAt),
		Headers:        maps.headers,
		Body:           maps.body,
		Query:          maps.query,
		Path:           maps.path,
		RawHeaders:     maps.rawHeaders,
		RawBody:        maps.rawBody,
		RawQuery:       maps.rawQuery,
		RawPath:        maps.rawPath,
		CreatedAt:      pgtype.Timestamptz{Time: filter.CreatedAt, Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: filter.UpdatedAt, Valid: true},
	})

	if err != nil {
		s.logger.Error("failed to create filter", "error", err)
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
		s.logger.Error("failed to start transaction", "error", err)
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
		setDefaultEnabledAt(filter)

		maps, err := prepareFilterMaps(filter)
		if err != nil {
			s.logger.Error("failed to prepare filter", "error", err)
			return util.NewServiceError(http.StatusBadRequest, err)
		}

		// Create filter
		err = qtx.CreateFilter(ctx, repo.CreateFilterParams{
			ID:             common.StringToPgText(filter.UID),
			SubscriptionID: common.StringToPgText(filter.SubscriptionID),
			EventType:      common.StringToPgText(filter.EventType),
			EnabledAt:      timeToPgTimestamptz(filter.EnabledAt),
			Headers:        maps.headers,
			Body:           maps.body,
			Query:          maps.query,
			Path:           maps.path,
			RawHeaders:     maps.rawHeaders,
			RawBody:        maps.rawBody,
			RawQuery:       maps.rawQuery,
			RawPath:        maps.rawPath,
			CreatedAt:      pgtype.Timestamptz{Time: filter.CreatedAt, Valid: true},
			UpdatedAt:      pgtype.Timestamptz{Time: filter.UpdatedAt, Valid: true},
		})

		if err != nil {
			s.logger.Error("failed to create filter", "error", err)
			return util.NewServiceError(http.StatusInternalServerError, errors.New("filter could not be created"))
		}
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		s.logger.Error("failed to commit transaction", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// UpdateFilter updates an existing filter
func (s *Service) UpdateFilter(ctx context.Context, filter *datastore.EventTypeFilter) error {
	if filter == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("filter cannot be nil"))
	}

	maps, err := prepareFilterMaps(filter)
	if err != nil {
		s.logger.Error("failed to prepare filter", "error", err)
		return util.NewServiceError(http.StatusBadRequest, err)
	}

	// Update filter
	rowsAffected, err := s.repo.UpdateFilter(ctx, repo.UpdateFilterParams{
		ID:         common.StringToPgText(filter.UID),
		EnabledAt:  timeToPgTimestamptz(filter.EnabledAt),
		Headers:    maps.headers,
		Body:       maps.body,
		Query:      maps.query,
		Path:       maps.path,
		RawHeaders: maps.rawHeaders,
		RawBody:    maps.rawBody,
		RawQuery:   maps.rawQuery,
		RawPath:    maps.rawPath,
		EventType:  common.StringToPgText(filter.EventType),
		UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})

	if err != nil {
		s.logger.Error("failed to update filter", "error", err)
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
		s.logger.Error("failed to start transaction", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Update each filter in the transaction
	for i := range filters {
		filter := &filters[i]

		maps, err := prepareFilterMaps(filter)
		if err != nil {
			s.logger.Error("failed to prepare filter", "error", err)
			return util.NewServiceError(http.StatusBadRequest, err)
		}

		filter.UpdatedAt = time.Now()

		// Update filter
		rowsAffected, err := qtx.UpdateFilter(ctx, repo.UpdateFilterParams{
			ID:         common.StringToPgText(filter.UID),
			EnabledAt:  timeToPgTimestamptz(filter.EnabledAt),
			Headers:    maps.headers,
			Body:       maps.body,
			Query:      maps.query,
			Path:       maps.path,
			RawHeaders: maps.rawHeaders,
			RawBody:    maps.rawBody,
			RawQuery:   maps.rawQuery,
			RawPath:    maps.rawPath,
			EventType:  common.StringToPgText(filter.EventType),
			UpdatedAt:  pgtype.Timestamptz{Time: filter.UpdatedAt, Valid: true},
		})

		if err != nil {
			s.logger.Errorf("failed to update filter %s: %v", filter.UID, err)
			return util.NewServiceError(http.StatusInternalServerError, fmt.Errorf("failed to update filter %s", filter.UID))
		}

		if rowsAffected == 0 {
			return util.NewServiceError(http.StatusNotFound, fmt.Errorf("filter %s not found", filter.UID))
		}
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		s.logger.Error("failed to commit transaction", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// DeleteFilter deletes a filter by ID
func (s *Service) DeleteFilter(ctx context.Context, filterID string) error {
	rowsAffected, err := s.repo.DeleteFilter(ctx, common.StringToPgText(filterID))
	if err != nil {
		s.logger.Error("failed to delete filter", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, errors.New("filter could not be deleted"))
	}

	if rowsAffected == 0 {
		return datastore.ErrFilterNotFound
	}

	return nil
}

// FindFilterByID retrieves a filter by its ID
func (s *Service) FindFilterByID(ctx context.Context, filterID string) (*datastore.EventTypeFilter, error) {
	row, err := s.repo.FindFilterByID(ctx, common.StringToPgText(filterID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrFilterNotFound
		}
		s.logger.Error("failed to find filter by id", "error", err)
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	filter, err := rowToEventTypeFilter(row)
	if err != nil {
		s.logger.Error("failed to convert row to filter", "error", err)
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return filter, nil
}

// FindFiltersBySubscriptionID retrieves all filters for a subscription
func (s *Service) FindFiltersBySubscriptionID(ctx context.Context, subscriptionID string) ([]datastore.EventTypeFilter, error) {
	rows, err := s.repo.FindFiltersBySubscriptionID(ctx, common.StringToPgText(subscriptionID))
	if err != nil {
		s.logger.Error("failed to find filters by subscription id", "error", err)
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	filters := make([]datastore.EventTypeFilter, 0, len(rows))
	for _, row := range rows {
		filter, err := rowToEventTypeFilter(row)
		if err != nil {
			s.logger.Error("failed to convert row to filter", "error", err)
			return nil, util.NewServiceError(http.StatusInternalServerError, err)
		}
		filters = append(filters, *filter)
	}

	return filters, nil
}

// FindFilterBySubscriptionAndEventType retrieves a filter by subscription and event type
func (s *Service) FindFilterBySubscriptionAndEventType(ctx context.Context, subscriptionID, eventType string) (*datastore.EventTypeFilter, error) {
	row, err := s.repo.FindFilterBySubscriptionAndEventType(ctx, repo.FindFilterBySubscriptionAndEventTypeParams{
		SubscriptionID: common.StringToPgText(subscriptionID),
		EventType:      common.StringToPgText(eventType),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrFilterNotFound
		}
		s.logger.Error("failed to find filter by subscription and event type", "error", err)
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	filter, err := rowToEventTypeFilter(row)
	if err != nil {
		s.logger.Error("failed to convert row to filter", "error", err)
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return filter, nil
}

// TestFilter tests if a request matches a filter
func (s *Service) TestFilter(ctx context.Context, subscriptionID, eventType string, payload any) (bool, error) {
	filter, hasInactiveFilter, err := s.findFilterForEventType(ctx, subscriptionID, eventType)
	if err != nil {
		return false, err
	}
	if filter == nil {
		if hasInactiveFilter {
			return false, nil
		}
		// No filtering, so it matches.
		return true, nil
	}

	// Empty filter means it matches everything
	if len(filter.Body) == 0 && len(filter.Headers) == 0 && len(filter.Query) == 0 && len(filter.Path) == 0 {
		return true, nil
	}

	req := normalizeFilterTestRequest(payload)
	return matchStoredFilter(req, filter)
}

func (s *Service) findFilterForEventType(ctx context.Context, subscriptionID, eventType string) (*datastore.EventTypeFilter, bool, error) {
	exactFilter, exactFilterExists, err := s.findFilter(ctx, subscriptionID, eventType)
	if err != nil {
		return nil, false, err
	}

	if exactFilterExists && exactFilter.IsEnabled() {
		return exactFilter, false, nil
	}

	if eventType == "*" {
		return nil, exactFilterExists, nil
	}

	wildcardFilter, wildcardFilterExists, err := s.findFilter(ctx, subscriptionID, "*")
	if err != nil {
		return nil, false, err
	}

	if wildcardFilterExists && wildcardFilter.IsEnabled() {
		return wildcardFilter, false, nil
	}

	return nil, exactFilterExists || wildcardFilterExists, nil
}

func (s *Service) findFilter(ctx context.Context, subscriptionID, eventType string) (*datastore.EventTypeFilter, bool, error) {
	filter, err := s.FindFilterBySubscriptionAndEventType(ctx, subscriptionID, eventType)
	if err != nil {
		if errors.Is(err, datastore.ErrFilterNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return filter, true, nil
}

func normalizeFilterTestRequest(payload any) datastore.FilterTestRequest {
	switch v := payload.(type) {
	case datastore.FilterTestRequest:
		return v
	case *datastore.FilterTestRequest:
		if v == nil {
			return datastore.FilterTestRequest{}
		}
		return *v
	default:
		return datastore.FilterTestRequest{Body: payload}
	}
}

func matchStoredFilter(req datastore.FilterTestRequest, filter *datastore.EventTypeFilter) (bool, error) {
	isBodyMatched, err := compareStoredFilterBody(req.Body, filter.Body)
	if err != nil || !isBodyMatched {
		return isBodyMatched, err
	}

	isHeaderMatched, err := compareStoredFilterScope(req.Headers, filter.Headers)
	if err != nil || !isHeaderMatched {
		return isHeaderMatched, err
	}

	isQueryMatched, err := compareStoredFilterScope(req.Query, filter.Query)
	if err != nil || !isQueryMatched {
		return isQueryMatched, err
	}

	return compareStoredFilterScope(req.Path, filter.Path)
}

func compareStoredFilterBody(payload any, filter datastore.M) (bool, error) {
	if len(filter) == 0 {
		return true, nil
	}

	if payload == nil {
		return false, nil
	}

	p, err := flatten.Flatten(payload)
	if err != nil {
		return false, err
	}

	return compare.Compare(p, filter)
}

func compareStoredFilterScope(payload, filter datastore.M) (bool, error) {
	if len(filter) == 0 {
		return true, nil
	}

	if datastore.HasArrayWildcardSelector(filter) {
		return false, nil
	}

	if len(payload) == 0 {
		return false, nil
	}

	flatPayload, err := common.FlattenM(payload)
	if err != nil {
		return false, err
	}

	return compare.Compare(flatPayload, filter)
}
