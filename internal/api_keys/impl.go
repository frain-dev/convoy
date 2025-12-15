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
	api_key_models "github.com/frain-dev/convoy/internal/api_keys/models"
	"github.com/frain-dev/convoy/internal/api_keys/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

// Service implements the API key Service using SQLc-generated queries
type Service struct {
	logger   log.StdLogger
	repo     repo.Querier      // SQLc-generated interface
	db       *pgxpool.Pool     // Connection pool
	legacyDB database.Database // For gradual migration if needed
}

// New creates a new API key Service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
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

// rowToAPIKey converts any SQLc-generated row struct to datastore.APIKey
func (s *Service) rowToAPIKey(row interface{}) datastore.APIKey {
	// Extract fields using a helper that works with all row types
	var (
		id, name, keyType, maskID           string
		roleType, roleProject, roleEndpoint string
		hash, salt, userID                  string
		createdAt, updatedAt                pgtype.Timestamptz
		expiresAt, deletedAt                pgtype.Timestamptz
	)

	switch r := row.(type) {
	case repo.FindAPIKeyByIDRow:
		id, name, keyType, maskID = r.ID, r.Name, r.KeyType, r.MaskID
		roleType, roleProject, roleEndpoint = r.RoleType, r.RoleProject, r.RoleEndpoint
		hash, salt, userID = r.Hash, r.Salt, r.UserID
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		expiresAt, deletedAt = r.ExpiresAt, r.DeletedAt
	case repo.FindAPIKeyByMaskIDRow:
		id, name, keyType, maskID = r.ID, r.Name, r.KeyType, r.MaskID
		roleType, roleProject, roleEndpoint = r.RoleType, r.RoleProject, r.RoleEndpoint
		hash, salt, userID = r.Hash, r.Salt, r.UserID
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		expiresAt, deletedAt = r.ExpiresAt, r.DeletedAt
	case repo.FindAPIKeyByHashRow:
		id, name, keyType, maskID = r.ID, r.Name, r.KeyType, r.MaskID
		roleType, roleProject, roleEndpoint = r.RoleType, r.RoleProject, r.RoleEndpoint
		hash, salt, userID = r.Hash, r.Salt, r.UserID
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		expiresAt, deletedAt = r.ExpiresAt, r.DeletedAt
	case repo.FindAPIKeyByProjectIDRow:
		id, name, keyType, maskID = r.ID, r.Name, r.KeyType, r.MaskID
		roleType, roleProject, roleEndpoint = r.RoleType, r.RoleProject, r.RoleEndpoint
		hash, salt, userID = r.Hash, r.Salt, r.UserID
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		expiresAt, deletedAt = r.ExpiresAt, r.DeletedAt
	case repo.FetchAPIKeysPaginatedRow:
		id, name, keyType, maskID = r.ID, r.Name, r.KeyType, r.MaskID
		roleType, roleProject, roleEndpoint = r.RoleType, r.RoleProject, r.RoleEndpoint
		hash, salt, userID = r.Hash, r.Salt, r.UserID
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		expiresAt, deletedAt = r.ExpiresAt, r.DeletedAt
	default:
		return datastore.APIKey{}
	}

	return datastore.APIKey{
		UID:       id,
		Name:      name,
		Type:      datastore.KeyType(keyType),
		MaskID:    maskID,
		Role:      paramsToRole(roleType, roleProject, roleEndpoint),
		Hash:      hash,
		Salt:      salt,
		UserID:    userID,
		ExpiresAt: pgTimestamptzToNullTime(expiresAt),
		DeletedAt: pgTimestamptzToNullTime(deletedAt),
		CreatedAt: createdAt.Time,
		UpdatedAt: updatedAt.Time,
	}
}

// ============================================================================
// Service Implementation
// ============================================================================

// CreateAPIKey creates a new API key
func (s *Service) CreateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
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
func (s *Service) UpdateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
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
func (s *Service) GetAPIKeyByID(ctx context.Context, id string) (*datastore.APIKey, error) {
	row, err := s.repo.FindAPIKeyByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, api_key_models.ErrAPIKeyNotFound
		}
		s.logger.WithError(err).Error("failed to find api key by id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	apiKey := s.rowToAPIKey(row)
	return &apiKey, nil
}

// GetAPIKeyByMaskID retrieves an API key by its mask ID
// CRITICAL: This method is used for API key authentication in NativeRealm
func (s *Service) GetAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	row, err := s.repo.FindAPIKeyByMaskID(ctx, maskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, api_key_models.ErrAPIKeyNotFound
		}
		s.logger.WithError(err).Error("failed to find api key by mask id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	apiKey := s.rowToAPIKey(row)
	return &apiKey, nil
}

// GetAPIKeyByHash retrieves an API key by its hash
func (s *Service) GetAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	row, err := s.repo.FindAPIKeyByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, api_key_models.ErrAPIKeyNotFound
		}
		s.logger.WithError(err).Error("failed to find api key by hash")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	apiKey := s.rowToAPIKey(row)
	return &apiKey, nil
}

// GetAPIKeyByProjectID retrieves an API key by its project ID
func (s *Service) GetAPIKeyByProjectID(ctx context.Context, projectID string) (*datastore.APIKey, error) {
	row, err := s.repo.FindAPIKeyByProjectID(ctx, stringToPgTextFilter(projectID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, api_key_models.ErrAPIKeyNotFound
		}
		s.logger.WithError(err).Error("failed to find api key by project id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	apiKey := s.rowToAPIKey(row)
	return &apiKey, nil
}

// RevokeAPIKeys revokes (soft deletes) multiple API keys
func (s *Service) RevokeAPIKeys(ctx context.Context, ids []string) error {
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
func (s *Service) LoadAPIKeysPaged(ctx context.Context, filter *api_key_models.ApiKeyFilter, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	// Determine direction for SQL query
	direction := "next"
	if pageable.Direction == datastore.Prev {
		direction = "prev"
	}

	// Check if we have an endpoint_ids filter
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

	pagination = pagination.Build(*pageable, ids)

	return apiKeys, *pagination, nil
}
