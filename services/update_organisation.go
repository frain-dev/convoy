package services

import (
	"context"
	"net/url"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

type UpdateOrganisationService struct {
	OrgRepo       datastore.OrganisationRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	Org           *datastore.Organisation
	Update        *datastore.OrganisationRequest
	Logger        log.Logger
}

func (os *UpdateOrganisationService) Run(ctx context.Context) (*datastore.Organisation, error) {
	err := util.Validate(os.Update)
	if err != nil {
		os.Logger.ErrorContext(ctx, "failed to validate organisation update - validate", "error", err)
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if len(os.Update.Name) > 0 {
		os.Org.Name = os.Update.Name
	}

	if len(os.Update.CustomDomain) > 0 {
		u, err := url.Parse(os.Update.CustomDomain)
		if err != nil {
			os.Logger.ErrorContext(ctx, "failed to validate hostname", "error", err)
			return nil, &ServiceError{ErrMsg: err.Error()}
		}

		if len(u.Host) == 0 {
			os.Logger.ErrorContext(ctx, "failed to validate hostname - malformatted url")
			return nil, &ServiceError{ErrMsg: "failed to validate hostname: malformatted url", Err: nil}
		}

		os.Org.CustomDomain = null.NewString(u.Host, true)
	}

	err = os.OrgRepo.UpdateOrganisation(ctx, os.Org)
	if err != nil {
		os.Logger.ErrorContext(ctx, "failed to to update organisation", "error", err)
		return nil, &ServiceError{ErrMsg: "failed to update organisation", Err: err}
	}

	return os.Org, nil
}
