package portal_links

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dchest/uniuri"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"
	"github.com/oklog/ulid/v2"
	"github.com/xdg-go/pbkdf2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/portal_links/models"
	"github.com/frain-dev/convoy/internal/portal_links/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

// ServiceError represents an error that occurs during service operations
type ServiceError struct {
	ErrMsg string
	Err    error
}

func (s *ServiceError) Error() string {
	return s.ErrMsg
}

func (s *ServiceError) Unwrap() error {
	return s.Err
}

type Service struct {
	logger       log.StdLogger
	repo         repo.Querier
	db           *pgxpool.Pool
	endpointRepo datastore.EndpointRepository
}

func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger:       logger,
		repo:         repo.New(db.GetConn()),
		db:           db.GetConn(),
		endpointRepo: postgres.NewEndpointRepo(db),
	}
}

// Helper function to convert []string to pgtype.Text (using pq.StringArray format)
func stringsToPgText(strs []string) pgtype.Text {
	if len(strs) == 0 {
		return pgtype.Text{String: "", Valid: false}
	}
	// Use pq.StringArray's Value() method to get the database representation
	arr := pq.StringArray(strs)
	val, err := arr.Value()
	if err != nil || val == nil {
		return pgtype.Text{String: "", Valid: false}
	}
	// The Value() returns a string in PostgreSQL array format
	if str, ok := val.(string); ok {
		return pgtype.Text{String: str, Valid: true}
	}
	return pgtype.Text{String: "", Valid: false}
}

// Helper function to convert pgtype.Text back to []string
func pgTextToStrings(pt pgtype.Text) []string {
	if !pt.Valid || pt.String == "" {
		return []string{}
	}
	// Parse pq.StringArray format
	var arr pq.StringArray
	if err := arr.Scan(pt.String); err != nil {
		return []string{}
	}
	return arr
}

// Helper function to convert []byte JSON to EndpointMetadata
func bytesToEndpointMetadata(b []byte) datastore.EndpointMetadata {
	var metadata datastore.EndpointMetadata
	if len(b) == 0 {
		return metadata
	}
	if err := json.Unmarshal(b, &metadata); err != nil {
		return datastore.EndpointMetadata{}
	}
	return metadata
}

// generateAuthKey generates a masked auth key for portal links
// Returns (maskId, fullKey) where fullKey is in format "PRT.maskId.secretKey"
func generateAuthKey() (string, string) {
	mask := uniuri.NewLen(16)
	key := uniuri.NewLen(64)

	var builder strings.Builder
	builder.WriteString(util.PortalAuthTokenPrefix)
	builder.WriteString(util.Separator)
	builder.WriteString(mask)
	builder.WriteString(util.Separator)
	builder.WriteString(key)

	return mask, builder.String()
}

// updateEndpointOwnerIDs updates the owner_id for the given endpoints and links them to the portal link
func (s *Service) updateEndpointOwnerIDs(ctx context.Context, qtx *repo.Queries, endpointIDs []string, portalLinkID, portalOwnerID, projectID string) error {
	for _, endpointID := range endpointIDs {
		endpoint, err := s.endpointRepo.FindEndpointByID(ctx, endpointID, projectID)
		if err != nil {
			return &ServiceError{ErrMsg: fmt.Sprintf("failed to find endpoint %s", endpointID), Err: err}
		}

		// If endpoint's owner_id is blank, set it to portal link's owner_id
		if util.IsStringEmpty(endpoint.OwnerID) {
			err = qtx.UpdateEndpointOwnerID(ctx, repo.UpdateEndpointOwnerIDParams{
				ID:        endpointID,
				ProjectID: projectID,
				OwnerID:   pgtype.Text{String: portalOwnerID, Valid: true},
			})
			if err != nil {
				return &ServiceError{ErrMsg: fmt.Sprintf("failed to update endpoint %s owner_id", endpointID), Err: err}
			}
		} else if endpoint.OwnerID != portalOwnerID {
			return &ServiceError{ErrMsg: fmt.Sprintf("endpoint %s already has owner_id %s", endpointID, endpoint.OwnerID)}
		}

		// Link endpoint to portal link
		err = qtx.CreatePortalLinkEndpoint(ctx, repo.CreatePortalLinkEndpointParams{
			PortalLinkID: portalLinkID,
			EndpointID:   endpointID,
		})
		if err != nil {
			return &ServiceError{ErrMsg: "failed to link endpoint to portal link", Err: err}
		}
	}
	return nil
}

func (s *Service) CreatePortalLink(ctx context.Context, projectId string, request *models.CreatePortalLinkRequest) (*datastore.PortalLink, error) {
	if err := request.Validate(); err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	token := uniuri.NewLen(24)
	uid := ulid.Make().String()
	if util.IsStringEmpty(request.OwnerID) {
		request.OwnerID = uid
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return nil, &ServiceError{ErrMsg: "failed to create portal link", Err: err}
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Create portal link
	err = qtx.CreatePortalLink(ctx, repo.CreatePortalLinkParams{
		ID:                uid,
		ProjectID:         projectId,
		Name:              request.Name,
		Token:             token,
		OwnerID:           pgtype.Text{String: request.OwnerID, Valid: true},
		AuthType:          repo.ConvoyPortalAuthTypes(request.AuthType),
		CanManageEndpoint: pgtype.Bool{Bool: request.CanManageEndpoint, Valid: true},
		Endpoints:         stringsToPgText(request.Endpoints),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to create portal link")
		return nil, &ServiceError{ErrMsg: "failed to create portal link", Err: err}
	}

	// Handle endpoints if provided
	if len(request.Endpoints) > 0 {
		err = s.updateEndpointOwnerIDs(ctx, qtx, request.Endpoints, uid, request.OwnerID, projectId)
		if err != nil {
			return nil, err
		}
	} else if !util.IsStringEmpty(request.OwnerID) {
		// Fetch endpoints by owner_id and link them
		endpoints, err2 := s.endpointRepo.FindEndpointsByOwnerID(ctx, projectId, request.OwnerID)
		if err2 != nil {
			return nil, &ServiceError{ErrMsg: "failed to fetch endpoints by owner_id", Err: err2}
		}

		for _, endpoint := range endpoints {
			err2 = qtx.CreatePortalLinkEndpoint(ctx, repo.CreatePortalLinkEndpointParams{
				PortalLinkID: uid,
				EndpointID:   endpoint.UID,
			})
			if err2 != nil {
				return nil, &ServiceError{ErrMsg: "failed to link endpoint to portal link", Err: err2}
			}
		}
	}

	// Generate auth key if auth type is refresh token
	var authKey string
	if datastore.PortalAuthType(request.AuthType) == datastore.PortalAuthTypeRefreshToken {
		portalToken, err := generateToken(uid)
		if err != nil {
			s.logger.WithError(err).Error("failed to generate auth token")
			return nil, &ServiceError{ErrMsg: "failed to generate auth token", Err: err}
		}

		// Create a portal link auth token
		err = qtx.CreatePortalLinkAuthToken(ctx, repo.CreatePortalLinkAuthTokenParams{
			ID:             portalToken.UID,
			PortalLinkID:   uid,
			TokenMaskID:    pgtype.Text{String: portalToken.MaskId, Valid: true},
			TokenHash:      pgtype.Text{String: portalToken.Hash, Valid: true},
			TokenSalt:      pgtype.Text{String: portalToken.Salt, Valid: true},
			TokenExpiresAt: pgtype.Timestamptz{Time: portalToken.ExpiresAt.Time, Valid: portalToken.ExpiresAt.Valid},
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to create portal link auth token")
			return nil, &ServiceError{ErrMsg: "failed to create auth token", Err: err}
		}

		authKey = portalToken.AuthKey
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return nil, &ServiceError{ErrMsg: "failed to create portal link", Err: err}
	}

	// Ensure endpoints is empty slice instead of nil
	endpoints := request.Endpoints
	if endpoints == nil {
		endpoints = []string{}
	}

	return &datastore.PortalLink{
		UID:               uid,
		Name:              request.Name,
		ProjectID:         projectId,
		OwnerID:           request.OwnerID,
		Token:             token,
		Endpoints:         endpoints,
		AuthType:          datastore.PortalAuthType(request.AuthType),
		CanManageEndpoint: request.CanManageEndpoint,
		AuthKey:           authKey,
	}, nil
}

func (s *Service) UpdatePortalLink(ctx context.Context, projectID string, portalLink *datastore.PortalLink, request *models.UpdatePortalLinkRequest) (*datastore.PortalLink, error) {
	if err := request.Validate(); err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return nil, &ServiceError{ErrMsg: "failed to update portal link", Err: err}
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Update portal link
	err = qtx.UpdatePortalLink(ctx, repo.UpdatePortalLinkParams{
		ID:                portalLink.UID,
		ProjectID:         projectID,
		Endpoints:         stringsToPgText(request.Endpoints),
		OwnerID:           pgtype.Text{String: request.OwnerID, Valid: true},
		CanManageEndpoint: pgtype.Bool{Bool: request.CanManageEndpoint, Valid: true},
		Name:              request.Name,
		AuthType:          repo.ConvoyPortalAuthTypes(request.AuthType),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to update portal link")
		return nil, &ServiceError{ErrMsg: "an error occurred while updating portal link", Err: err}
	}

	// Delete existing endpoint links
	err = qtx.DeletePortalLinkEndpoints(ctx, repo.DeletePortalLinkEndpointsParams{
		PortalLinkID: portalLink.UID,
		EndpointID:   "",
	})
	if err != nil {
		return nil, &ServiceError{ErrMsg: "failed to delete portal link endpoints", Err: err}
	}

	// Handle new endpoints
	if len(request.Endpoints) > 0 {
		err = s.updateEndpointOwnerIDs(ctx, qtx, request.Endpoints, portalLink.UID, request.OwnerID, projectID)
		if err != nil {
			return nil, err
		}
	} else if !util.IsStringEmpty(request.OwnerID) {
		endpoints, err2 := s.endpointRepo.FindEndpointsByOwnerID(ctx, projectID, request.OwnerID)
		if err2 != nil {
			return nil, &ServiceError{ErrMsg: "failed to fetch endpoints by owner_id", Err: err2}
		}

		for _, endpoint := range endpoints {
			err2 = qtx.CreatePortalLinkEndpoint(ctx, repo.CreatePortalLinkEndpointParams{
				PortalLinkID: portalLink.UID,
				EndpointID:   endpoint.UID,
			})
			if err2 != nil {
				return nil, &ServiceError{ErrMsg: "failed to link endpoint to portal link", Err: err2}
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return nil, &ServiceError{ErrMsg: "failed to update portal link", Err: err}
	}

	// Ensure endpoints is empty slice instead of nil
	endpoints := request.Endpoints
	if endpoints == nil {
		endpoints = []string{}
	}

	portalLink.Name = request.Name
	portalLink.OwnerID = request.OwnerID
	portalLink.AuthType = datastore.PortalAuthType(request.AuthType)
	portalLink.CanManageEndpoint = request.CanManageEndpoint
	portalLink.Endpoints = endpoints

	return portalLink, nil
}

func (s *Service) GetPortalLink(ctx context.Context, projectID, portalLinkID string) (*datastore.PortalLink, error) {
	row, err := s.repo.FetchPortalLinkById(ctx, repo.FetchPortalLinkByIdParams{
		ID:        portalLinkID,
		ProjectID: projectID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
		}
		s.logger.WithError(err).Error("failed to fetch portal link")
		return nil, &ServiceError{ErrMsg: "error retrieving portal link", Err: err}
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return nil, &ServiceError{ErrMsg: "failed to create portal link", Err: err}
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Generate auth key if auth type is refresh token
	var authKey string
	if datastore.PortalAuthType(row.AuthType) == datastore.PortalAuthTypeRefreshToken {
		portalToken, err := generateToken(portalLinkID)
		if err != nil {
			s.logger.WithError(err).Error("failed to generate auth token")
			return nil, &ServiceError{ErrMsg: "failed to generate auth token", Err: err}
		}

		// Create a portal link auth token
		err = qtx.CreatePortalLinkAuthToken(ctx, repo.CreatePortalLinkAuthTokenParams{
			ID:             portalToken.UID,
			PortalLinkID:   portalLinkID,
			TokenMaskID:    pgtype.Text{String: portalToken.MaskId, Valid: true},
			TokenHash:      pgtype.Text{String: portalToken.Hash, Valid: true},
			TokenSalt:      pgtype.Text{String: portalToken.Salt, Valid: true},
			TokenExpiresAt: pgtype.Timestamptz{Time: portalToken.ExpiresAt.Time, Valid: portalToken.ExpiresAt.Valid},
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to create portal link auth token")
			return nil, &ServiceError{ErrMsg: "failed to create auth token", Err: err}
		}

		authKey = portalToken.AuthKey
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return nil, &ServiceError{ErrMsg: "failed to create portal link", Err: err}
	}

	// Ensure endpoints is never nil
	endpoints := pgTextToStrings(row.Endpoints)
	if endpoints == nil {
		endpoints = []string{}
	}

	return &datastore.PortalLink{
		UID:               row.ID,
		ProjectID:         row.ProjectID,
		Name:              row.Name,
		Token:             row.Token,
		Endpoints:         endpoints,
		AuthKey:           authKey,
		AuthType:          datastore.PortalAuthType(row.AuthType),
		CanManageEndpoint: row.CanManageEndpoint,
		OwnerID:           row.OwnerID,
		EndpointCount:     int(row.EndpointCount.Int64),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
		EndpointsMetadata: bytesToEndpointMetadata(row.EndpointsMetadata),
	}, nil
}

func (s *Service) GetPortalLinkByToken(ctx context.Context, token string) (*datastore.PortalLink, error) {
	row, err := s.repo.FetchPortalLinkByToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
		}
		s.logger.WithError(err).Error("failed to fetch portal link by token")
		return nil, &ServiceError{ErrMsg: "error retrieving portal link", Err: err}
	}

	// Ensure endpoints is never nil
	endpoints := pgTextToStrings(row.Endpoints)
	if endpoints == nil {
		endpoints = []string{}
	}

	return &datastore.PortalLink{
		UID:               row.ID,
		ProjectID:         row.ProjectID,
		Name:              row.Name,
		Token:             row.Token,
		Endpoints:         endpoints,
		AuthType:          datastore.PortalAuthType(row.AuthType),
		CanManageEndpoint: row.CanManageEndpoint,
		OwnerID:           row.OwnerID,
		EndpointCount:     int(row.EndpointCount.Int64),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
		EndpointsMetadata: bytesToEndpointMetadata(row.EndpointsMetadata),
	}, nil
}

func (s *Service) GetPortalLinkByOwnerID(ctx context.Context, projectID, ownerID string) (*datastore.PortalLink, error) {
	row, err := s.repo.FetchPortalLinkByOwnerID(ctx, repo.FetchPortalLinkByOwnerIDParams{
		OwnerID:   pgtype.Text{String: ownerID, Valid: true},
		ProjectID: projectID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
		}
		s.logger.WithError(err).Error("failed to fetch portal link by owner ID")
		return nil, &ServiceError{ErrMsg: "error retrieving portal link", Err: err}
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return nil, &ServiceError{ErrMsg: "failed to create portal link", Err: err}
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Generate auth key if auth type is refresh token
	var authKey string
	if datastore.PortalAuthType(row.AuthType) == datastore.PortalAuthTypeRefreshToken {
		portalToken, err := generateToken(row.ID)
		if err != nil {
			s.logger.WithError(err).Error("failed to generate auth token")
			return nil, &ServiceError{ErrMsg: "failed to generate auth token", Err: err}
		}

		// Create a portal link auth token
		err = qtx.CreatePortalLinkAuthToken(ctx, repo.CreatePortalLinkAuthTokenParams{
			ID:             portalToken.UID,
			PortalLinkID:   row.ID,
			TokenMaskID:    pgtype.Text{String: portalToken.MaskId, Valid: true},
			TokenHash:      pgtype.Text{String: portalToken.Hash, Valid: true},
			TokenSalt:      pgtype.Text{String: portalToken.Salt, Valid: true},
			TokenExpiresAt: pgtype.Timestamptz{Time: portalToken.ExpiresAt.Time, Valid: portalToken.ExpiresAt.Valid},
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to create portal link auth token")
			return nil, &ServiceError{ErrMsg: "failed to create auth token", Err: err}
		}

		authKey = portalToken.AuthKey
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return nil, &ServiceError{ErrMsg: "failed to create portal link", Err: err}
	}

	// Ensure endpoints is never nil
	endpoints := pgTextToStrings(row.Endpoints)
	if endpoints == nil {
		endpoints = []string{}
	}

	return &datastore.PortalLink{
		UID:               row.ID,
		ProjectID:         row.ProjectID,
		Name:              row.Name,
		Token:             row.Token,
		Endpoints:         endpoints,
		AuthKey:           authKey,
		AuthType:          datastore.PortalAuthType(row.AuthType),
		CanManageEndpoint: row.CanManageEndpoint,
		OwnerID:           row.OwnerID,
		EndpointCount:     int(row.EndpointCount.Int64),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
		EndpointsMetadata: bytesToEndpointMetadata(row.EndpointsMetadata),
	}, nil
}

func (s *Service) RefreshPortalLinkAuthToken(ctx context.Context, projectID, portalLinkID string) (*datastore.PortalLink, error) {
	// Fetch the portal link to ensure it exists
	portalLink, err := s.GetPortalLink(ctx, projectID, portalLinkID)
	if err != nil {
		return nil, err
	}

	// Generate new auth key
	maskId, key := generateAuthKey()

	// Generate salt
	salt, err := util.GenerateSecret()
	if err != nil {
		s.logger.WithError(err).Error("failed to generate salt")
		return nil, &ServiceError{ErrMsg: "failed to generate auth token", Err: err}
	}

	// Create hash using PBKDF2
	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	// Set expiry time (1 hour from now)
	expiresAt := time.Now().Add(time.Hour)

	// Start transaction to insert auth token
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return nil, &ServiceError{ErrMsg: "failed to refresh auth token", Err: err}
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Create a portal link auth token
	err = qtx.CreatePortalLinkAuthToken(ctx, repo.CreatePortalLinkAuthTokenParams{
		ID:             ulid.Make().String(),
		PortalLinkID:   portalLinkID,
		TokenMaskID:    pgtype.Text{String: maskId, Valid: true},
		TokenHash:      pgtype.Text{String: encodedKey, Valid: true},
		TokenSalt:      pgtype.Text{String: salt, Valid: true},
		TokenExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to create portal link auth token")
		return nil, &ServiceError{ErrMsg: "failed to refresh auth token", Err: err}
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return nil, &ServiceError{ErrMsg: "failed to refresh auth token", Err: err}
	}

	// Set the auth key on the portal link (this is the plain text key to return to the user)
	portalLink.AuthKey = key

	return portalLink, nil
}

func (s *Service) RevokePortalLink(ctx context.Context, projectID, portalLinkID string) error {
	err := s.repo.DeletePortalLink(ctx, repo.DeletePortalLinkParams{
		ID:        portalLinkID,
		ProjectID: projectID,
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to revoke portal link")
		return &ServiceError{ErrMsg: "failed to revoke portal link", Err: err}
	}
	return nil
}

func (s *Service) LoadPortalLinksPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	// Consolidate endpoint IDs from filter
	var endpointIDs []string
	if !util.IsStringEmpty(filter.EndpointID) {
		endpointIDs = append(endpointIDs, filter.EndpointID)
	}
	if len(filter.EndpointIDs) > 0 {
		endpointIDs = append(endpointIDs, filter.EndpointIDs...)
	}

	// Determine direction for query
	direction := "next"
	if pageable.Direction == datastore.Prev {
		direction = "prev"
	}

	// Query portal links with pagination
	rows, err := s.repo.FetchPortalLinksPaginated(ctx, repo.FetchPortalLinksPaginatedParams{
		Direction:         direction,
		ProjectID:         projectID,
		Cursor:            pageable.Cursor(),
		HasEndpointFilter: len(endpointIDs) > 0,
		EndpointIds:       endpointIDs,
		LimitVal:          int64(pageable.Limit()),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to load portal links paged")
		return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: "an error occurred while fetching portal links", Err: err}
	}

	// Convert rows to portal links
	portalLinks := make([]datastore.PortalLink, 0, len(rows))
	for _, row := range rows {
		endpoints := pgTextToStrings(row.Endpoints)
		if endpoints == nil {
			endpoints = []string{}
		}

		portalLinks = append(portalLinks, datastore.PortalLink{
			UID:               row.ID,
			ProjectID:         row.ProjectID,
			Name:              row.Name,
			Token:             row.Token,
			Endpoints:         endpoints,
			AuthType:          datastore.PortalAuthType(row.AuthType),
			CanManageEndpoint: row.CanManageEndpoint,
			OwnerID:           row.OwnerID,
			EndpointCount:     int(row.EndpointCount.Int64),
			CreatedAt:         row.CreatedAt.Time,
			UpdatedAt:         row.UpdatedAt.Time,
			EndpointsMetadata: bytesToEndpointMetadata(row.EndpointsMetadata),
		})
	}

	// Build IDs for pagination
	ids := make([]string, len(portalLinks))
	for i := range portalLinks {
		ids[i] = portalLinks[i].UID
	}

	// If we got more results than requested, trim the extra one (used for hasNext detection)
	if len(portalLinks) > pageable.PerPage {
		portalLinks = portalLinks[:len(portalLinks)-1]
	}

	// Count previous rows for pagination
	var prevRowCount datastore.PrevRowCount
	if len(portalLinks) > 0 {
		first := portalLinks[0]
		count, err := s.repo.CountPrevPortalLinks(ctx, repo.CountPrevPortalLinksParams{
			ProjectID:         projectID,
			Cursor:            first.UID,
			HasEndpointFilter: len(endpointIDs) > 0,
			EndpointIds:       endpointIDs,
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to count prev portal links")
			return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: "an error occurred while counting portal links", Err: err}
		}
		prevRowCount.Count = int(count.Int64)
	}

	// Build pagination data
	pagination := &datastore.PaginationData{PrevRowCount: prevRowCount}
	pagination = pagination.Build(pageable, ids)

	// Generate auth tokens for portal links that need them (non-static token types)
	if len(portalLinks) > 0 {
		var authTokens []datastore.PortalToken
		for i := range portalLinks {
			if portalLinks[i].AuthType == datastore.PortalAuthTypeStaticToken {
				continue
			}

			// Generate auth token
			portalToken, err := generateToken(portalLinks[i].UID)
			if err != nil {
				s.logger.WithError(err).Error("failed to generate auth token")
				return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: "failed to generate auth tokens", Err: err}
			}

			authTokens = append(authTokens, *portalToken)

			// Set the auth key on the portal link so it's returned to the caller
			portalLinks[i].AuthKey = portalToken.AuthKey
		}

		// Bulk insert auth tokens if any were generated
		if len(authTokens) > 0 {
			tx, err := s.db.Begin(ctx)
			if err != nil {
				s.logger.WithError(err).Error("failed to start transaction for auth tokens")
				return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: "failed to generate auth tokens", Err: err}
			}
			defer tx.Rollback(ctx)

			qtx := repo.New(tx)
			for _, token := range authTokens {
				err = qtx.CreatePortalLinkAuthToken(ctx, repo.CreatePortalLinkAuthTokenParams{
					ID:             token.UID,
					PortalLinkID:   token.PortalLinkID,
					TokenMaskID:    pgtype.Text{String: token.MaskId, Valid: true},
					TokenHash:      pgtype.Text{String: token.Hash, Valid: true},
					TokenSalt:      pgtype.Text{String: token.Salt, Valid: true},
					TokenExpiresAt: pgtype.Timestamptz{Time: token.ExpiresAt.Time, Valid: token.ExpiresAt.Valid},
				})
				if err != nil {
					s.logger.WithError(err).Error("failed to create portal link auth token")
					return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: "failed to generate auth tokens", Err: err}
				}
			}

			if err = tx.Commit(ctx); err != nil {
				s.logger.WithError(err).Error("failed to commit auth tokens transaction")
				return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: "failed to generate auth tokens", Err: err}
			}
		}
	}

	return portalLinks, *pagination, nil
}

func (s *Service) FindPortalLinksByOwnerID(ctx context.Context, ownerID string) ([]datastore.PortalLink, error) {
	rows, err := s.repo.FetchPortalLinksByOwnerID(ctx, pgtype.Text{String: ownerID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &ServiceError{ErrMsg: "portal links not found", Err: datastore.ErrPortalLinkNotFound}
		}
		s.logger.WithError(err).Error("failed to fetch portal links by owner ID")
		return nil, &ServiceError{ErrMsg: "error retrieving portal links", Err: err}
	}

	// Convert rows to portal links
	portalLinks := make([]datastore.PortalLink, 0, len(rows))
	for _, row := range rows {
		endpoints := pgTextToStrings(row.Endpoints)
		if endpoints == nil {
			endpoints = []string{}
		}

		portalLinks = append(portalLinks, datastore.PortalLink{
			UID:               row.ID,
			ProjectID:         row.ProjectID,
			Name:              row.Name,
			Token:             row.Token,
			Endpoints:         endpoints,
			AuthType:          datastore.PortalAuthType(row.AuthType),
			CanManageEndpoint: row.CanManageEndpoint,
			OwnerID:           row.OwnerID,
			EndpointCount:     int(row.EndpointCount.Int64),
			CreatedAt:         row.CreatedAt.Time,
			UpdatedAt:         row.UpdatedAt.Time,
			EndpointsMetadata: bytesToEndpointMetadata(row.EndpointsMetadata),
		})
	}

	// Generate auth tokens for portal links that need them (non-static token types)
	if len(portalLinks) > 0 {
		var authTokens []datastore.PortalToken
		for i := range portalLinks {
			if portalLinks[i].AuthType == datastore.PortalAuthTypeStaticToken {
				continue
			}

			// Generate auth token
			portalToken, err := generateToken(portalLinks[i].UID)
			if err != nil {
				s.logger.WithError(err).Error("failed to generate auth token")
				return nil, &ServiceError{ErrMsg: "failed to generate auth tokens", Err: err}
			}

			authTokens = append(authTokens, *portalToken)

			// Set the auth key on the portal link so it's returned to the caller
			portalLinks[i].AuthKey = portalToken.AuthKey
		}

		// Bulk insert auth tokens if any were generated
		if len(authTokens) > 0 {
			tx, err := s.db.Begin(ctx)
			if err != nil {
				s.logger.WithError(err).Error("failed to start transaction for auth tokens")
				return nil, &ServiceError{ErrMsg: "failed to generate auth tokens", Err: err}
			}
			defer tx.Rollback(ctx)

			qtx := repo.New(tx)
			for _, token := range authTokens {
				err = qtx.CreatePortalLinkAuthToken(ctx, repo.CreatePortalLinkAuthTokenParams{
					ID:             token.UID,
					PortalLinkID:   token.PortalLinkID,
					TokenMaskID:    pgtype.Text{String: token.MaskId, Valid: true},
					TokenHash:      pgtype.Text{String: token.Hash, Valid: true},
					TokenSalt:      pgtype.Text{String: token.Salt, Valid: true},
					TokenExpiresAt: pgtype.Timestamptz{Time: token.ExpiresAt.Time, Valid: token.ExpiresAt.Valid},
				})
				if err != nil {
					s.logger.WithError(err).Error("failed to create portal link auth token")
					return nil, &ServiceError{ErrMsg: "failed to generate auth tokens", Err: err}
				}
			}

			if err = tx.Commit(ctx); err != nil {
				s.logger.WithError(err).Error("failed to commit auth tokens transaction")
				return nil, &ServiceError{ErrMsg: "failed to generate auth tokens", Err: err}
			}
		}
	}

	return portalLinks, nil
}

func (s *Service) FindPortalLinkByMaskId(ctx context.Context, maskId string) (*datastore.PortalLink, error) {
	row, err := s.repo.FetchPortalLinkByMaskId(ctx, pgtype.Text{String: maskId, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
		}
		s.logger.WithError(err).Error("failed to fetch portal link by mask ID")
		return nil, &ServiceError{ErrMsg: "error retrieving portal link", Err: err}
	}

	// Ensure endpoints is never nil
	endpoints := pgTextToStrings(row.Endpoints)
	if endpoints == nil {
		endpoints = []string{}
	}

	return &datastore.PortalLink{
		UID:               row.ID,
		ProjectID:         row.ProjectID,
		Name:              row.Name,
		Token:             row.Token,
		Endpoints:         endpoints,
		AuthType:          datastore.PortalAuthType(row.AuthType),
		OwnerID:           row.OwnerID,
		EndpointCount:     int(row.EndpointCount.Int64),
		TokenSalt:         row.TokenSalt.String,
		TokenMaskId:       row.TokenMaskID.String,
		TokenHash:         row.TokenHash.String,
		CanManageEndpoint: row.CanManageEndpoint,
		TokenExpiresAt:    null.NewTime(row.TokenExpiresAt.Time, row.TokenExpiresAt.Valid),
	}, nil
}

func generateToken(portalLinkId string) (*datastore.PortalToken, error) {
	maskId, key := generateAuthKey()
	salt, err := util.GenerateSecret()
	if err != nil {
		return nil, err
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	portalToken := &datastore.PortalToken{
		UID:          ulid.Make().String(),
		PortalLinkID: portalLinkId,
		MaskId:       maskId,
		Hash:         encodedKey,
		Salt:         salt,
		AuthKey:      key,
		ExpiresAt:    null.NewTime(time.Now().Add(time.Hour), true),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return portalToken, nil
}
