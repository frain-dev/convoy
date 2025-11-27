package services

import (
	"context"
	"errors"
	"time"

	"github.com/dchest/uniuri"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var ErrInvalidEndpoints = errors.New("endpoints cannot be empty")

type CreatePortalLinkService struct {
	PortalLinkRepo        datastore.PortalLinkRepository
	EndpointRepo          datastore.EndpointRepository
	UpdateEndpointOwnerID bool

	Portal  *models.CreatePortalLinkRequest
	Project *datastore.Project
}

func (p *CreatePortalLinkService) Run(ctx context.Context) (*datastore.PortalLink, error) {
	if err := util.Validate(p.Portal); err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if err := p.Portal.Validate(); err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	uid := ulid.Make().String()
	if util.IsStringEmpty(p.Portal.OwnerID) {
		p.Portal.OwnerID = uid
	}

	portalLink := &datastore.PortalLink{
		UID:               uid,
		ProjectID:         p.Project.UID,
		Name:              p.Portal.Name,
		Token:             uniuri.NewLen(24),
		OwnerID:           p.Portal.OwnerID,
		AuthType:          datastore.PortalAuthType(p.Portal.AuthType),
		CanManageEndpoint: p.Portal.CanManageEndpoint,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	err := p.PortalLinkRepo.CreatePortalLink(ctx, portalLink, p.UpdateEndpointOwnerID, p.Portal.Endpoints)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create portal link")
		return nil, &ServiceError{ErrMsg: "failed to create portal link"}
	}

	return portalLink, nil
}
