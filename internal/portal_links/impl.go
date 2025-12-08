package portal_links

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/portal_links/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type Service struct {
	logger         log.StdLogger
	repo           repo.Querier
	portalLinkRepo datastore.PortalLinkRepository
}

func New(logger log.StdLogger, db *pgxpool.Pool) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db),
	}
}

// NewWithPostgresRepo creates a new Service with both SQLc and legacy postgres repos
// This is temporary during the migration period
func NewWithPostgresRepo(logger log.StdLogger, db *pgxpool.Pool, pg database.Database) *Service {
	return &Service{
		logger:         logger,
		repo:           repo.New(db),
		portalLinkRepo: postgres.NewPortalLinkRepo(pg),
	}
}

func (s *Service) CreatePortalLink(ctx context.Context, projectId string, request *models.CreatePortalLinkRequest) (*datastore.PortalLink, error) {
	if err := request.Validate(); err != nil {
		return nil, &services.ServiceError{ErrMsg: err.Error()}
	}

	uid := ulid.Make().String()
	if util.IsStringEmpty(request.OwnerID) {
		request.OwnerID = uid
	}

	// Build the portal link
	portalLink := &datastore.PortalLink{
		UID:               uid,
		ProjectID:         projectId,
		Name:              request.Name,
		Token:             uniuri.NewLen(24),
		OwnerID:           request.OwnerID,
		Endpoints:         request.Endpoints,
		AuthType:          datastore.PortalAuthType(request.AuthType),
		CanManageEndpoint: request.CanManageEndpoint,
	}

	// Use the legacy repo for creation as it handles complex endpoint logic
	if s.portalLinkRepo != nil {
		err := s.portalLinkRepo.CreatePortalLink(ctx, portalLink)
		if err != nil {
			s.logger.WithError(err).Error("failed to create portal link")
			return nil, &services.ServiceError{ErrMsg: "failed to create portal link"}
		}
		return portalLink, nil
	}

	return nil, &services.ServiceError{ErrMsg: "portal link repository not initialized"}
}

func (s *Service) UpdatePortalLink(ctx context.Context, projectID string, portalLink *datastore.PortalLink, request *models.UpdatePortalLinkRequest) (*datastore.PortalLink, error) {
	if err := request.Validate(); err != nil {
		return nil, &services.ServiceError{ErrMsg: err.Error()}
	}

	// Update the portal link fields
	portalLink.Name = request.Name
	portalLink.OwnerID = request.OwnerID
	portalLink.AuthType = datastore.PortalAuthType(request.AuthType)
	portalLink.CanManageEndpoint = request.CanManageEndpoint
	portalLink.Endpoints = request.Endpoints

	// Use the legacy repo for now since it has the full update logic with endpoints
	if s.portalLinkRepo != nil {
		err := s.portalLinkRepo.UpdatePortalLink(ctx, projectID, portalLink)
		if err != nil {
			s.logger.WithError(err).Error("failed to update portal link")
			return nil, &services.ServiceError{ErrMsg: "an error occurred while updating portal link"}
		}
		return portalLink, nil
	}

	return nil, &services.ServiceError{ErrMsg: "portal link repository not initialized"}
}

func (s *Service) GetPortalLink(ctx context.Context, projectID, portalLinkID string) (*datastore.PortalLink, error) {
	if s.portalLinkRepo != nil {
		portalLink, err := s.portalLinkRepo.FindPortalLinkByID(ctx, projectID, portalLinkID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, datastore.ErrPortalLinkNotFound) {
				return nil, &services.ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
			}
			s.logger.WithError(err).Error("failed to fetch portal link")
			return nil, &services.ServiceError{ErrMsg: "error retrieving portal link", Err: err}
		}
		return portalLink, nil
	}

	return nil, &services.ServiceError{ErrMsg: "portal link repository not initialized"}
}

func (s *Service) GetPortalLinkByToken(ctx context.Context, token string) (*datastore.PortalLink, error) {
	if s.portalLinkRepo != nil {
		portalLink, err := s.portalLinkRepo.FindPortalLinkByToken(ctx, token)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, datastore.ErrPortalLinkNotFound) {
				return nil, &services.ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
			}
			s.logger.WithError(err).Error("failed to fetch portal link by token")
			return nil, &services.ServiceError{ErrMsg: "error retrieving portal link", Err: err}
		}
		return portalLink, nil
	}

	return nil, &services.ServiceError{ErrMsg: "portal link repository not initialized"}
}

func (s *Service) GetPortalLinkByOwnerID(ctx context.Context, projectID, ownerID string) (*datastore.PortalLink, error) {
	if s.portalLinkRepo != nil {
		portalLink, err := s.portalLinkRepo.FindPortalLinkByOwnerID(ctx, projectID, ownerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, datastore.ErrPortalLinkNotFound) {
				return nil, &services.ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
			}
			s.logger.WithError(err).Error("failed to fetch portal link by owner ID")
			return nil, &services.ServiceError{ErrMsg: "error retrieving portal link", Err: err}
		}
		return portalLink, nil
	}

	return nil, &services.ServiceError{ErrMsg: "portal link repository not initialized"}
}

func (s *Service) RefreshPortalLinkAuthToken(ctx context.Context, projectID, portalLinkID string) (*datastore.PortalLink, error) {
	if s.portalLinkRepo != nil {
		portalLink, err := s.portalLinkRepo.RefreshPortalLinkAuthToken(ctx, projectID, portalLinkID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, datastore.ErrPortalLinkNotFound) {
				return nil, &services.ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
			}
			s.logger.WithError(err).Error("failed to refresh portal link auth token")
			return nil, &services.ServiceError{ErrMsg: "error refreshing portal link auth token", Err: err}
		}
		return portalLink, nil
	}

	return nil, &services.ServiceError{ErrMsg: "portal link repository not initialized"}
}

func (s *Service) RevokePortalLink(ctx context.Context, projectID, portalLinkID string) error {
	if s.portalLinkRepo != nil {
		// First verify the portal link exists
		_, err := s.portalLinkRepo.FindPortalLinkByID(ctx, projectID, portalLinkID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, datastore.ErrPortalLinkNotFound) {
				return &services.ServiceError{ErrMsg: "portal link not found", Err: datastore.ErrPortalLinkNotFound}
			}
			s.logger.WithError(err).Error("failed to find portal link for revocation")
			return &services.ServiceError{ErrMsg: "error retrieving portal link", Err: err}
		}

		// Revoke the portal link
		err = s.portalLinkRepo.RevokePortalLink(ctx, projectID, portalLinkID)
		if err != nil {
			s.logger.WithError(err).Error("failed to revoke portal link")
			return &services.ServiceError{ErrMsg: "failed to revoke portal link", Err: err}
		}
		return nil
	}

	return &services.ServiceError{ErrMsg: "portal link repository not initialized"}
}

func (s *Service) LoadPortalLinksPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	if s.portalLinkRepo != nil {
		portalLinks, paginationData, err := s.portalLinkRepo.LoadPortalLinksPaged(ctx, projectID, filter, pageable)
		if err != nil {
			s.logger.WithError(err).Error("failed to load portal links paged")
			return nil, datastore.PaginationData{}, &services.ServiceError{ErrMsg: "an error occurred while fetching portal links"}
		}
		return portalLinks, paginationData, nil
	}

	return nil, datastore.PaginationData{}, &services.ServiceError{ErrMsg: "portal link repository not initialized"}
}
