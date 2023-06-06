package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
)

var ErrInvalidEndpoints = errors.New("endpoints cannot be empty")

type CreatePortalLinkService struct {
	PortalLinkRepo datastore.PortalLinkRepository
	EndpointRepo   datastore.EndpointRepository

	Portal  *models.PortalLink
	Project *datastore.Project
}

func (p *CreatePortalLinkService) Run(ctx context.Context) (*datastore.PortalLink, error) {
	if err := util.Validate(p.Portal); err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if len(p.Portal.Endpoints) == 0 {
		return nil, &ServiceError{ErrMsg: ErrInvalidEndpoints.Error()}
	}

	if err := findEndpoints(ctx, p.Portal.Endpoints, p.Project, p.EndpointRepo); err != nil {
		return nil, err
	}

	portalLink := &datastore.PortalLink{
		UID:       ulid.Make().String(),
		ProjectID: p.Project.UID,
		Name:      p.Portal.Name,
		Token:     uniuri.NewLen(24),
		Endpoints: p.Portal.Endpoints,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := p.PortalLinkRepo.CreatePortalLink(ctx, portalLink)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create portal link")
		return nil, &ServiceError{ErrMsg: "failed to create portal link"}
	}

	return portalLink, nil
}

func findEndpoints(ctx context.Context, endpoints []string, project *datastore.Project, endpointRepo datastore.EndpointRepository) error {
	for _, e := range endpoints {
		endpoint, err := endpointRepo.FindEndpointByID(ctx, e, project.UID)
		log.FromContext(ctx).WithError(err).Errorf("failed to find endpoint %s", e)
		if errors.Is(err, datastore.ErrEndpointNotFound) {
			return &ServiceError{ErrMsg: fmt.Sprintf("endpoint with ID :%s not found", e), Err: err}
		}

		if endpoint.ProjectID != project.UID {
			return &ServiceError{ErrMsg: fmt.Sprintf("unauthorized access to endpoint with ID: %s", e)}
		}
	}

	return nil
}
