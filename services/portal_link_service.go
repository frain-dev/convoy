package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrInvalidEndpoints = errors.New("endpoints cannot be empty")

type PortalLinkService struct {
	portalLinkRepo  datastore.PortalLinkRepository
	endpointService *EndpointService
	endpointRepo    datastore.EndpointRepository
}

func NewPortalLinkService(portalLinkRepo datastore.PortalLinkRepository, endpointService *EndpointService) *PortalLinkService {
	return &PortalLinkService{
		portalLinkRepo:  portalLinkRepo,
		endpointService: endpointService,
		endpointRepo:    endpointService.endpointRepo,
	}
}

func (p *PortalLinkService) CreatePortalLink(ctx context.Context, portal *models.PortalLink, group *datastore.Project) (*datastore.PortalLink, error) {
	if err := util.Validate(portal); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if len(portal.Endpoints) == 0 {
		return nil, util.NewServiceError(http.StatusBadRequest, ErrInvalidEndpoints)
	}

	if err := p.findEndpoints(ctx, portal.Endpoints, group); err != nil {
		return nil, err
	}

	portalLink := &datastore.PortalLink{
		UID:       uuid.New().String(),
		GroupID:   group.UID,
		Name:      portal.Name,
		Token:     uniuri.NewLen(24),
		Endpoints: portal.Endpoints,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err := p.portalLinkRepo.CreatePortalLink(ctx, portalLink)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create portal link"))
	}

	return portalLink, nil
}

func (p *PortalLinkService) UpdatePortalLink(ctx context.Context, group *datastore.Project, update *models.PortalLink, portalLink *datastore.PortalLink) (*datastore.PortalLink, error) {
	if err := util.Validate(update); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if len(update.Endpoints) == 0 {
		return nil, util.NewServiceError(http.StatusBadRequest, ErrInvalidEndpoints)
	}

	if err := p.findEndpoints(ctx, update.Endpoints, group); err != nil {
		return nil, err
	}

	portalLink.Name = update.Name
	portalLink.Endpoints = update.Endpoints
	err := p.portalLinkRepo.UpdatePortalLink(ctx, group.UID, portalLink)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating portal link"))
	}

	return portalLink, nil
}

func (p *PortalLinkService) FindPortalLinkByID(ctx context.Context, group *datastore.Project, uid string) (*datastore.PortalLink, error) {
	portalLink, err := p.portalLinkRepo.FindPortalLinkByID(ctx, group.UID, uid)
	if err != nil {
		if err == datastore.ErrPortalLinkNotFound {
			return nil, util.NewServiceError(http.StatusNotFound, err)
		}

		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("error retrieving portal link"))
	}

	return portalLink, nil
}

func (p *PortalLinkService) LoadPortalLinksPaged(ctx context.Context, group *datastore.Project, f *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	portalLinks, paginationData, err := p.portalLinkRepo.LoadPortalLinksPaged(ctx, group.UID, f, pageable)
	if err != nil {
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching portal links"))
	}

	return portalLinks, paginationData, nil
}

func (p *PortalLinkService) RevokePortalLink(ctx context.Context, group *datastore.Project, portalLink *datastore.PortalLink) error {
	err := p.portalLinkRepo.RevokePortalLink(ctx, group.UID, portalLink.UID)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to delete portal link"))
	}

	return nil
}

func (p *PortalLinkService) CreateEndpoint(ctx context.Context, group *datastore.Project, data models.Endpoint, portalLink *datastore.PortalLink) (*datastore.Endpoint, error) {
	endpoint, err := p.endpointService.CreateEndpoint(ctx, data, group.UID)
	if err != nil {
		return nil, err
	}

	portalLink.Endpoints = append(portalLink.Endpoints, endpoint.UID)
	err = p.portalLinkRepo.UpdatePortalLink(ctx, group.UID, portalLink)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating portal link"))
	}

	return endpoint, nil
}

func (p *PortalLinkService) GetPortalLinkEndpoints(ctx context.Context, portal *datastore.PortalLink) ([]datastore.Endpoint, error) {
	endpoints, err := p.endpointRepo.FindEndpointsByID(ctx, portal.Endpoints)
	if err != nil {
		return nil, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching endpoints"))
	}

	return endpoints, err
}

func (p *PortalLinkService) findEndpoints(ctx context.Context, endpoints []string, group *datastore.Project) error {
	for _, e := range endpoints {
		endpoint, err := p.endpointRepo.FindEndpointByID(ctx, e)
		if errors.Is(err, datastore.ErrEndpointNotFound) {
			return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("endpoint with ID :%s not found", e))
		}

		if endpoint.GroupID != group.UID {
			return util.NewServiceError(http.StatusForbidden, fmt.Errorf("unauthorized access to endpoint with ID: %s", e))
		}
	}

	return nil
}
