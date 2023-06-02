package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/cache"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
)

var ErrInvalidEndpoints = errors.New("endpoints cannot be empty")

type PortalLinkService struct {
	portalLinkRepo datastore.PortalLinkRepository
	endpointRepo   datastore.EndpointRepository
	cache          cache.Cache
	projectRepo    datastore.ProjectRepository
}

func NewPortalLinkService(portalLinkRepo datastore.PortalLinkRepository, endpointRepo datastore.EndpointRepository, cache cache.Cache, projectRepo datastore.ProjectRepository) *PortalLinkService {
	return &PortalLinkService{
		portalLinkRepo: portalLinkRepo,
		endpointRepo:   endpointRepo,
		cache:          cache,
		projectRepo:    projectRepo,
	}
}

func (p *PortalLinkService) CreatePortalLink(ctx context.Context, portal *models.PortalLink, project *datastore.Project) (*datastore.PortalLink, error) {
	if err := util.Validate(portal); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if len(portal.Endpoints) == 0 {
		return nil, util.NewServiceError(http.StatusBadRequest, ErrInvalidEndpoints)
	}

	if err := p.findEndpoints(ctx, portal.Endpoints, project); err != nil {
		return nil, err
	}

	portalLink := &datastore.PortalLink{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		Name:      portal.Name,
		Token:     uniuri.NewLen(24),
		Endpoints: portal.Endpoints,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := p.portalLinkRepo.CreatePortalLink(ctx, portalLink)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create portal link"))
	}

	return portalLink, nil
}

func (p *PortalLinkService) UpdatePortalLink(ctx context.Context, project *datastore.Project, update *models.PortalLink, portalLink *datastore.PortalLink) (*datastore.PortalLink, error) {
	if err := util.Validate(update); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if len(update.Endpoints) == 0 {
		return nil, util.NewServiceError(http.StatusBadRequest, ErrInvalidEndpoints)
	}

	if err := p.findEndpoints(ctx, update.Endpoints, project); err != nil {
		return nil, err
	}

	portalLink.Name = update.Name
	portalLink.Endpoints = update.Endpoints
	err := p.portalLinkRepo.UpdatePortalLink(ctx, project.UID, portalLink)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating portal link"))
	}

	return portalLink, nil
}

func (p *PortalLinkService) CreateEndpoint(ctx context.Context, project *datastore.Project, data models.CreateEndpoint, portalLink *datastore.PortalLink) (*datastore.Endpoint, error) {
	ce := CreateEndpointService{
		Cache:        p.cache,
		EndpointRepo: p.endpointRepo,
		ProjectRepo:  p.projectRepo,
		E:            data,
		ProjectID:    project.UID,
	}

	endpoint, err := ce.Run(ctx)
	if err != nil {
		return nil, err
	}

	portalLink.Endpoints = append(portalLink.Endpoints, endpoint.UID)
	err = p.portalLinkRepo.UpdatePortalLink(ctx, project.UID, portalLink)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating portal link"))
	}

	return endpoint, nil
}

func (p *PortalLinkService) findEndpoints(ctx context.Context, endpoints []string, project *datastore.Project) error {
	for _, e := range endpoints {
		endpoint, err := p.endpointRepo.FindEndpointByID(ctx, e, project.UID)
		if errors.Is(err, datastore.ErrEndpointNotFound) {
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("endpoint with ID :%s not found", e))
		}

		if endpoint.ProjectID != project.UID {
			return util.NewServiceError(http.StatusForbidden, fmt.Errorf("unauthorized access to endpoint with ID: %s", e))
		}
	}

	return nil
}
