package portal_links

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dchest/uniuri"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/portal_links/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

type Service struct {
	logger       log.StdLogger
	repo         repo.Querier
	db           *pgxpool.Pool
	endpointRepo datastore.EndpointRepository
	legacyDB     database.Database
}

func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger:       logger,
		repo:         repo.New(db.GetConn()),
		db:           db.GetConn(),
		endpointRepo: postgres.NewEndpointRepo(db),
		legacyDB:     db,
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

// updateEndpointOwnerIDs updates the owner_id for the given endpoints and links them to the portal link
func (s *Service) updateEndpointOwnerIDs(ctx context.Context, qtx *repo.Queries, endpointIDs []string, portalLinkID, portalOwnerID, projectID string) error {
	for _, endpointID := range endpointIDs {
		endpoint, err := s.endpointRepo.FindEndpointByID(ctx, endpointID, projectID)
		if err != nil {
			return &services.ServiceError{ErrMsg: fmt.Sprintf("failed to find endpoint %s", endpointID), Err: err}
		}

		// If endpoint's owner_id is blank, set it to portal link's owner_id
		if util.IsStringEmpty(endpoint.OwnerID) {
			err = qtx.UpdateEndpointOwnerID(ctx, repo.UpdateEndpointOwnerIDParams{
				ID:        endpointID,
				ProjectID: projectID,
				OwnerID:   pgtype.Text{String: portalOwnerID, Valid: true},
			})
			if err != nil {
				return &services.ServiceError{ErrMsg: fmt.Sprintf("failed to update endpoint %s owner_id", endpointID), Err: err}
			}
		} else if endpoint.OwnerID != portalOwnerID {
			return &services.ServiceError{ErrMsg: fmt.Sprintf("endpoint %s already has owner_id %s", endpointID, endpoint.OwnerID)}
		}

		// Link endpoint to portal link
		err = qtx.CreatePortalLinkEndpoint(ctx, repo.CreatePortalLinkEndpointParams{
			PortalLinkID: portalLinkID,
			EndpointID:   endpointID,
		})
		if err != nil {
			return &services.ServiceError{ErrMsg: "failed to link endpoint to portal link", Err: err}
		}
	}
	return nil
}

func (s *Service) CreatePortalLink(ctx context.Context, projectId string, request *models.CreatePortalLinkRequest) (*datastore.PortalLink, error) {
	if err := request.Validate(); err != nil {
		return nil, &services.ServiceError{ErrMsg: err.Error()}
	}

	uid := ulid.Make().String()
	if util.IsStringEmpty(request.OwnerID) {
		request.OwnerID = uid
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return nil, &services.ServiceError{ErrMsg: "failed to create portal link", Err: err}
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Create portal link
	err = qtx.CreatePortalLink(ctx, repo.CreatePortalLinkParams{
		ID:                uid,
		ProjectID:         projectId,
		Name:              request.Name,
		Token:             uniuri.NewLen(24),
		OwnerID:           pgtype.Text{String: request.OwnerID, Valid: true},
		AuthType:          repo.ConvoyPortalAuthTypes(request.AuthType),
		CanManageEndpoint: pgtype.Bool{Bool: request.CanManageEndpoint, Valid: true},
		Endpoints:         stringsToPgText(request.Endpoints),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to create portal link")
		return nil, &services.ServiceError{ErrMsg: "failed to create portal link", Err: err}
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
			return nil, &services.ServiceError{ErrMsg: "failed to fetch endpoints by owner_id", Err: err2}
		}

		for _, endpoint := range endpoints {
			err2 = qtx.CreatePortalLinkEndpoint(ctx, repo.CreatePortalLinkEndpointParams{
				PortalLinkID: uid,
				EndpointID:   endpoint.UID,
			})
			if err2 != nil {
				return nil, &services.ServiceError{ErrMsg: "failed to link endpoint to portal link", Err: err2}
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return nil, &services.ServiceError{ErrMsg: "failed to create portal link", Err: err}
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
		Endpoints:         endpoints,
		AuthType:          datastore.PortalAuthType(request.AuthType),
		CanManageEndpoint: request.CanManageEndpoint,
	}, nil
}

func (s *Service) UpdatePortalLink(ctx context.Context, projectID string, portalLink *datastore.PortalLink, request *models.UpdatePortalLinkRequest) (*datastore.PortalLink, error) {
	if err := request.Validate(); err != nil {
		return nil, &services.ServiceError{ErrMsg: err.Error()}
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return nil, &services.ServiceError{ErrMsg: "failed to update portal link", Err: err}
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
		return nil, &services.ServiceError{ErrMsg: "an error occurred while updating portal link", Err: err}
	}

	// Delete existing endpoint links
	err = qtx.DeletePortalLinkEndpoints(ctx, repo.DeletePortalLinkEndpointsParams{
		PortalLinkID: portalLink.UID,
		EndpointID:   "",
	})
	if err != nil {
		return nil, &services.ServiceError{ErrMsg: "failed to delete portal link endpoints", Err: err}
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
			return nil, &services.ServiceError{ErrMsg: "failed to fetch endpoints by owner_id", Err: err2}
		}

		for _, endpoint := range endpoints {
			err2 = qtx.CreatePortalLinkEndpoint(ctx, repo.CreatePortalLinkEndpointParams{
				PortalLinkID: portalLink.UID,
				EndpointID:   endpoint.UID,
			})
			if err2 != nil {
				return nil, &services.ServiceError{ErrMsg: "failed to link endpoint to portal link", Err: err2}
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return nil, &services.ServiceError{ErrMsg: "failed to update portal link", Err: err}
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
			return nil, &services.ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
		}
		s.logger.WithError(err).Error("failed to fetch portal link")
		return nil, &services.ServiceError{ErrMsg: "error retrieving portal link", Err: err}
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

func (s *Service) GetPortalLinkByToken(ctx context.Context, token string) (*datastore.PortalLink, error) {
	row, err := s.repo.FetchPortalLinkByToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &services.ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
		}
		s.logger.WithError(err).Error("failed to fetch portal link by token")
		return nil, &services.ServiceError{ErrMsg: "error retrieving portal link", Err: err}
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
			return nil, &services.ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
		}
		s.logger.WithError(err).Error("failed to fetch portal link by owner ID")
		return nil, &services.ServiceError{ErrMsg: "error retrieving portal link", Err: err}
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

func (s *Service) RefreshPortalLinkAuthToken(ctx context.Context, projectID, portalLinkID string) (*datastore.PortalLink, error) {
	// For now, delegate to the old repo as this requires complex token generation logic
	// This can be migrated later
	portalLinkRepo := postgres.NewPortalLinkRepo(s.legacyDB)
	portalLink, err := portalLinkRepo.RefreshPortalLinkAuthToken(ctx, projectID, portalLinkID)
	if err != nil {
		if errors.Is(err, datastore.ErrPortalLinkNotFound) {
			return nil, &services.ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
		}
		s.logger.WithError(err).Error("failed to refresh portal link auth token")
		return nil, &services.ServiceError{ErrMsg: "error refreshing portal link auth token", Err: err}
	}
	return portalLink, nil
}

func (s *Service) RevokePortalLink(ctx context.Context, projectID, portalLinkID string) error {
	err := s.repo.DeletePortalLink(ctx, repo.DeletePortalLinkParams{
		ID:        portalLinkID,
		ProjectID: projectID,
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to revoke portal link")
		return &services.ServiceError{ErrMsg: "failed to revoke portal link", Err: err}
	}
	return nil
}

func (s *Service) LoadPortalLinksPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	// For now, delegate to the old repo as pagination is complex
	// This can be migrated later
	portalLinkRepo := postgres.NewPortalLinkRepo(s.legacyDB)
	portalLinks, paginationData, err := portalLinkRepo.LoadPortalLinksPaged(ctx, projectID, filter, pageable)
	if err != nil {
		s.logger.WithError(err).Error("failed to load portal links paged")
		return nil, datastore.PaginationData{}, &services.ServiceError{ErrMsg: "an error occurred while fetching portal links", Err: err}
	}
	return portalLinks, paginationData, nil
}
