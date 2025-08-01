package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"
)

type CreateOrganisationService struct {
	OrgRepo       datastore.OrganisationRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	NewOrg        *models.Organisation
	User          *datastore.User
	Licenser      license.Licenser
	RoleType      auth.RoleType
}

var ErrOrgLimit = errors.New("your instance has reached it's organisation limit, upgrade to create new organisations")

func (co *CreateOrganisationService) Run(ctx context.Context) (*datastore.Organisation, error) {
	ok, err := co.Licenser.CreateOrg(ctx)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if !ok {
		return nil, &ServiceError{ErrMsg: ErrOrgLimit.Error(), Err: ErrOrgLimit}
	}

	err = util.Validate(co.NewOrg)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if len(co.NewOrg.Name) == 0 {
		log.FromContext(ctx).WithError(err).Error("organisation name is required")
		return nil, &ServiceError{ErrMsg: "organisation name is required", Err: err}
	}

	org := &datastore.Organisation{
		UID:       ulid.Make().String(),
		OwnerID:   co.User.UID,
		Name:      co.NewOrg.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	cfg, err := config.Get()
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to load config")
		return nil, &ServiceError{ErrMsg: "failed to create organisation", Err: err}
	}

	if len(cfg.CustomDomainSuffix) > 0 {
		org.AssignedDomain = null.NewString(fmt.Sprintf("%s.%s", uniuri.New(), cfg.CustomDomainSuffix), true)
	}

	err = co.OrgRepo.CreateOrganisation(ctx, org)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create organisation")
		return nil, &ServiceError{ErrMsg: "failed to create organisation", Err: err}
	}

	// Check if this is the first user (no instance admins exist yet)
	count, err := co.OrgMemberRepo.CountInstanceAdminUsers(ctx)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to count instance admin users")
		return nil, &ServiceError{ErrMsg: "failed to create organisation", Err: err}
	}

	// Only assign RoleInstanceAdmin to the first user, otherwise assign RoleOrganisationAdmin
	var roleType auth.RoleType
	if count == 0 {
		roleType = auth.RoleInstanceAdmin
	} else {
		roleType = auth.RoleOrganisationAdmin
	}

	_, err = NewOrganisationMemberService(co.OrgMemberRepo, co.Licenser).CreateOrganisationMember(ctx, org, co.User, &auth.Role{Type: roleType})
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create super_user member for organisation owner")
	}

	return org, nil
}
