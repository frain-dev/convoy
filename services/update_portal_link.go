package services

import (
	"context"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type UpdatePortalLinkService struct {
	PortalLinkRepo datastore.PortalLinkRepository
	EndpointRepo   datastore.EndpointRepository

	Project    *datastore.Project
	Update     *models.PortalLink
	PortalLink *datastore.PortalLink
}

func (p *UpdatePortalLinkService) Run(ctx context.Context) (*datastore.PortalLink, error) {
	if err := util.Validate(p.Update); err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if err := validateOwnerIdAndEndpoints(p.Update); err != nil {
		return nil, &ServiceError{ErrMsg: ErrInvalidEndpoints.Error()}
	}

	if err := findEndpoints(ctx, p.Update.Endpoints, p.Project, p.EndpointRepo); err != nil {
		return nil, err
	}

	p.PortalLink.Name = p.Update.Name
	p.PortalLink.OwnerID = p.Update.OwnerID
	p.PortalLink.Endpoints = p.Update.Endpoints
	p.PortalLink.CanManageEndpoint = p.Update.CanManageEndpoint
	err := p.PortalLinkRepo.UpdatePortalLink(ctx, p.Project.UID, p.PortalLink)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update portal link")
		return nil, &ServiceError{ErrMsg: "an error occurred while updating portal link"}
	}

	return p.PortalLink, nil
}
