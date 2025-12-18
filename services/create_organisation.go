package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dchest/uniuri"
	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type CreateOrganisationService struct {
	OrgRepo       datastore.OrganisationRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	NewOrg        *datastore.OrganisationRequest
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

	_, err = NewOrganisationMemberService(co.OrgMemberRepo, co.Licenser).CreateOrganisationMember(ctx, org, co.User, &auth.Role{Type: auth.RoleOrganisationAdmin})
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create super_user member for organisation owner")
	}

	if cfg.Billing.Enabled && co.Licenser.BillingModule() {
		go func() {
			billingCtx := context.Background()
			billingClient := billing.NewClient(cfg.Billing)

			if cfg.Host != "" {
				orgData := map[string]interface{}{
					"name":          org.Name,
					"external_id":   org.UID,
					"billing_email": "",
					"host":          cfg.Host,
				}

				_, createErr := billingClient.CreateOrganisation(billingCtx, orgData)
				if createErr != nil {
					// Log error but don't fail organisation creation if billing creation fails
					log.FromContext(billingCtx).WithError(createErr).Warn("failed to create organisation in billing service")
				} else {
					log.FromContext(billingCtx).Info("organisation created in billing service")
				}
			} else {
				log.FromContext(billingCtx).Warn("billing organisation creation skipped: host not configured")
			}
		}()
	}

	return org, nil
}
