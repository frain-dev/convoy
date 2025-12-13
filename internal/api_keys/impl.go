package api_keys

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

// Service implements the API key service using SQLc-generated queries
type service struct {
	logger   log.StdLogger
	repo     repo.Querier      // SQLc-generated interface
	db       *pgxpool.Pool     // Connection pool
	legacyDB database.Database // For gradual migration if needed
}

// New creates a new API key service
func New(logger log.StdLogger, db database.Database) Service {
	return &service{
		logger:   logger,
		repo:     repo.New(db.GetConn()),
		db:       db.GetConn(),
		legacyDB: db,
	}
}

// ============================================================================
// Type Conversion Helpers
// ============================================================================

// roleToParams converts auth.Role to database column parameters
func roleToParams(role auth.Role) (roleType, roleProject, roleEndpoint pgtype.Text) {
	roleType = pgtype.Text{
		String: string(role.Type),
		Valid:  !util.IsStringEmpty(string(role.Type)),
	}
	roleProject = pgtype.Text{
		String: role.Project,
		Valid:  !util.IsStringEmpty(role.Project),
	}
	roleEndpoint = pgtype.Text{
		String: role.Endpoint,
		Valid:  !util.IsStringEmpty(role.Endpoint),
	}
	return
}

// paramsToRole converts database columns to auth.Role
func paramsToRole(roleType, roleProject, roleEndpoint string) auth.Role {
	return auth.Role{
		Type:     auth.RoleType(roleType),
		Project:  roleProject,
		Endpoint: roleEndpoint,
	}
}

// nullTimeToPgTimestamptz converts null.Time to pgtype.Timestamptz
func nullTimeToPgTimestamptz(t null.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.Time, Valid: t.Valid}
}

// pgTimestamptzToNullTime converts pgtype.Timestamptz to null.Time
func pgTimestamptzToNullTime(t pgtype.Timestamptz) null.Time {
	return null.NewTime(t.Time, t.Valid)
}

// Helper to convert string to pgtype.Text for filters (empty string means no filter)
func stringToPgTextFilter(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// rowToAPIKey converts a Find query row to datastore.APIKey
func (s *service) rowToAPIKey(row interface{}) datastore.APIKey {
	// Handle different row types from SQLc
	switch r := row.(type) {
	case repo.FindAPIKeyByIDRow:
		return datastore.APIKey{
			UID:       r.ID,
			Name:      r.Name,
			Type:      datastore.KeyType(r.KeyType),
			MaskID:    r.MaskID,
			Role:      paramsToRole(r.RoleType, r.RoleProject, r.RoleEndpoint),
			Hash:      r.Hash,
			Salt:      r.Salt,
			UserID:    r.UserID,
			ExpiresAt: pgTimestamptzToNullTime(r.ExpiresAt),
			CreatedAt: r.CreatedAt.Time,
			UpdatedAt: r.UpdatedAt.Time,
		}
	case repo.FindAPIKeyByMaskIDRow:
		return datastore.APIKey{
			UID:       r.ID,
			Name:      r.Name,
			Type:      datastore.KeyType(r.KeyType),
			MaskID:    r.MaskID,
			Role:      paramsToRole(r.RoleType, r.RoleProject, r.RoleEndpoint),
			Hash:      r.Hash,
			Salt:      r.Salt,
			UserID:    r.UserID,
			ExpiresAt: pgTimestamptzToNullTime(r.ExpiresAt),
			CreatedAt: r.CreatedAt.Time,
			UpdatedAt: r.UpdatedAt.Time,
		}
	case repo.FindAPIKeyByHashRow:
		return datastore.APIKey{
			UID:       r.ID,
			Name:      r.Name,
			Type:      datastore.KeyType(r.KeyType),
			MaskID:    r.MaskID,
			Role:      paramsToRole(r.RoleType, r.RoleProject, r.RoleEndpoint),
			Hash:      r.Hash,
			Salt:      r.Salt,
			UserID:    r.UserID,
			ExpiresAt: pgTimestamptzToNullTime(r.ExpiresAt),
			CreatedAt: r.CreatedAt.Time,
			UpdatedAt: r.UpdatedAt.Time,
		}
	case repo.FindAPIKeyByProjectIDRow:
		return datastore.APIKey{
			UID:       r.ID,
			Name:      r.Name,
			Type:      datastore.KeyType(r.KeyType),
			MaskID:    r.MaskID,
			Role:      paramsToRole(r.RoleType, r.RoleProject, r.RoleEndpoint),
			Hash:      r.Hash,
			Salt:      r.Salt,
			UserID:    r.UserID,
			ExpiresAt: pgTimestamptzToNullTime(r.ExpiresAt),
			CreatedAt: r.CreatedAt.Time,
			UpdatedAt: r.UpdatedAt.Time,
		}
	case repo.FetchAPIKeysPaginatedRow:
		return datastore.APIKey{
			UID:       r.ID,
			Name:      r.Name,
			Type:      datastore.KeyType(r.KeyType),
			MaskID:    r.MaskID,
			Role:      paramsToRole(r.RoleType, r.RoleProject, r.RoleEndpoint),
			Hash:      r.Hash,
			Salt:      r.Salt,
			UserID:    r.UserID,
			ExpiresAt: pgTimestamptzToNullTime(r.ExpiresAt),
			CreatedAt: r.CreatedAt.Time,
			UpdatedAt: r.UpdatedAt.Time,
		}
	default:
		// Return empty APIKey if type is not recognized
		return datastore.APIKey{}
	}
}

// ============================================================================
// Service Implementation
// ============================================================================

// CreateAPIKey creates a new API key
func (s *service) CreateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	if apiKey == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("api key cannot be nil"))
	}

	// Convert role to database params
	roleType, roleProject, roleEndpoint := roleToParams(apiKey.Role)

	// Convert user_id to pgtype.Text (can be null)
	userID := pgtype.Text{
		String: apiKey.UserID,
		Valid:  !util.IsStringEmpty(apiKey.UserID),
	}

	// Create API key
	err := s.repo.CreateAPIKey(ctx, repo.CreateAPIKeyParams{
		ID:           apiKey.UID,
		Name:         apiKey.Name,
		KeyType:      string(apiKey.Type),
		MaskID:       apiKey.MaskID,
		RoleType:     roleType,
		RoleProject:  roleProject,
		RoleEndpoint: roleEndpoint,
		Hash:         apiKey.Hash,
		Salt:         apiKey.Salt,
		UserID:       userID,
		ExpiresAt:    nullTimeToPgTimestamptz(apiKey.ExpiresAt),
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to create api key")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// UpdateAPIKey updates an existing API key
func (s *service) UpdateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	if apiKey == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("api key cannot be nil"))
	}

	// Convert role to database params
	roleType, roleProject, roleEndpoint := roleToParams(apiKey.Role)

	// Update API key
	err := s.repo.UpdateAPIKey(ctx, repo.UpdateAPIKeyParams{
		ID:           apiKey.UID,
		Name:         apiKey.Name,
		RoleType:     roleType,
		RoleProject:  roleProject,
		RoleEndpoint: roleEndpoint,
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to update api key")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// GetAPIKeyByID retrieves an API key by its ID
func (s *service) GetAPIKeyByID(ctx context.Context, id string) (*datastore.APIKey, error) {
	row, err := s.repo.FindAPIKeyByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrAPIKeyNotFound
		}
		s.logger.WithError(err).Error("failed to find api key by id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	apiKey := s.rowToAPIKey(row)
	return &apiKey, nil
}

// GetAPIKeyByMaskID retrieves an API key by its mask ID
// CRITICAL: This method is used for API key authentication in NativeRealm
func (s *service) GetAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	row, err := s.repo.FindAPIKeyByMaskID(ctx, maskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrAPIKeyNotFound
		}
		s.logger.WithError(err).Error("failed to find api key by mask id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	apiKey := s.rowToAPIKey(row)
	return &apiKey, nil
}

// GetAPIKeyByHash retrieves an API key by its hash
func (s *service) GetAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	row, err := s.repo.FindAPIKeyByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrAPIKeyNotFound
		}
		s.logger.WithError(err).Error("failed to find api key by hash")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	apiKey := s.rowToAPIKey(row)
	return &apiKey, nil
}

// GetAPIKeyByProjectID retrieves an API key by its project ID
func (s *service) GetAPIKeyByProjectID(ctx context.Context, projectID string) (*datastore.APIKey, error) {
	row, err := s.repo.FindAPIKeyByProjectID(ctx, stringToPgTextFilter(projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrAPIKeyNotFound
		}
		s.logger.WithError(err).Error("failed to find api key by project id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	apiKey := s.rowToAPIKey(row)
	return &apiKey, nil
}

// RevokeAPIKeys revokes (soft deletes) multiple API keys
func (s *service) RevokeAPIKeys(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil // No-op for empty array
	}

	err := s.repo.RevokeAPIKeys(ctx, ids)
	if err != nil {
		s.logger.WithError(err).Error("failed to revoke api keys")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// LoadAPIKeysPaged retrieves API keys with pagination and filtering
func (s *service) LoadAPIKeysPaged(ctx context.Context, filter *datastore.ApiKeyFilter, pageable datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	// Determine direction for SQL query
	direction := "next"
	if pageable.Direction == datastore.Prev {
		direction = "prev"
	}

	// Check if we have endpoint_ids filter
	hasEndpointIdsFilter := len(filter.EndpointIDs) > 0

	// Convert filter strings to pgtype.Text
	projectID := stringToPgTextFilter(filter.ProjectID)
	endpointID := stringToPgTextFilter(filter.EndpointID)
	userID := stringToPgTextFilter(filter.UserID)
	keyType := stringToPgTextFilter(string(filter.KeyType))

	// Query with unified pagination
	rows, err := s.repo.FetchAPIKeysPaginated(ctx, repo.FetchAPIKeysPaginatedParams{
		Direction:      direction,
		Cursor:         stringToPgTextFilter(pageable.Cursor()),
		ProjectID:      projectID,
		EndpointID:     endpointID,
		UserID:         userID,
		KeyType:        keyType,
		HasEndpointIds: hasEndpointIdsFilter,
		EndpointIds:    filter.EndpointIDs,
		LimitVal:       int64(pageable.Limit()),
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to load api keys paged")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Convert rows to domain objects
	apiKeys := make([]datastore.APIKey, 0, len(rows))
	for _, row := range rows {
		apiKeys = append(apiKeys, s.rowToAPIKey(row))
	}

	// Detect hasNext by checking if we got more than requested
	hasNext := false
	if len(apiKeys) > pageable.PerPage {
		hasNext = true
		apiKeys = apiKeys[:pageable.PerPage] // Trim to actual page size
	}

	// Get first and last items for cursor values
	var first, last datastore.APIKey
	if len(apiKeys) > 0 {
		first = apiKeys[0]
		last = apiKeys[len(apiKeys)-1]
	}

	// Count previous rows for pagination metadata
	var prevRowCount datastore.PrevRowCount
	if len(apiKeys) > 0 {
		count, err := s.repo.CountPrevAPIKeys(ctx, repo.CountPrevAPIKeysParams{
			Cursor:         first.UID,
			ProjectID:      projectID,
			EndpointID:     endpointID,
			UserID:         userID,
			KeyType:        keyType,
			HasEndpointIds: hasEndpointIdsFilter,
			EndpointIds:    filter.EndpointIDs,
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to count prev api keys")
			return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err)
		}
		prevRowCount.Count = int(count.Int64)
	}

	// Build pagination metadata
	ids := make([]string, len(apiKeys))
	for i := range apiKeys {
		ids[i] = apiKeys[i].UID
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

	pagination = pagination.Build(pageable, ids)

	return apiKeys, *pagination, nil
}
