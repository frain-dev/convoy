package services

import (
	"context"
	"net/url"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"gopkg.in/guregu/null.v4"
)

type UpdateOrganisationService struct {
	OrgRepo       datastore.OrganisationRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	Org           *datastore.Organisation
	Update        *models.Organisation
}

func (os *UpdateOrganisationService) Run(ctx context.Context) (*datastore.Organisation, error) {
	err := util.Validate(os.Update)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to validate organisation update - validate")
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if len(os.Update.Name) > 0 {
		os.Org.Name = os.Update.Name
	}

	if len(os.Update.CustomDomain) > 0 {
		u, err := url.Parse(os.Update.CustomDomain)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to validate hostname")
			return nil, &ServiceError{ErrMsg: err.Error()}
		}

		if len(u.Host) == 0 {
			log.FromContext(ctx).Error("failed to validate hostname - malformatted url")
			return nil, &ServiceError{ErrMsg: "failed to validate hostname: malformatted url", Err: err}
		}

		os.Org.CustomDomain = null.NewString(u.Host, true)
	}

	err = os.OrgRepo.UpdateOrganisation(ctx, os.Org)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to to update organisation")
		return nil, &ServiceError{ErrMsg: "failed to update organisation", Err: err}
	}

	return os.Org, nil
}
