package services

import (
	"context"
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/cache"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type CreateEndpointPortalLinkService struct {
	PortalLinkRepo datastore.PortalLinkRepository
	EndpointRepo   datastore.EndpointRepository
	Cache          cache.Cache
	ProjectRepo    datastore.ProjectRepository

	Project    *datastore.Project
	Data       models.CreateEndpoint
	PortalLink *datastore.PortalLink
}

func (p *CreateEndpointPortalLinkService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	ce := CreateEndpointService{
		EndpointRepo: p.EndpointRepo,
		ProjectRepo:  p.ProjectRepo,
		E:            p.Data,
		ProjectID:    p.Project.UID,
	}

	endpoint, err := ce.Run(ctx)
	if err != nil {
		return nil, err
	}

	p.PortalLink.Endpoints = append(p.PortalLink.Endpoints, endpoint.UID)
	err = p.PortalLinkRepo.UpdatePortalLink(ctx, p.Project.UID, p.PortalLink)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating portal link"))
	}

	return endpoint, nil
}
