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
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
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
	ok, err := co.Licenser.CheckOrgLimit(ctx)
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

	hostForBilling := cfg.Host
	if cfg.Billing.OrganisationHost != "" {
		hostForBilling = cfg.Billing.OrganisationHost
	}
	if cfg.Billing.Enabled && hostForBilling != "" {
		orgCopy := *org
		cfgCopy := cfg
		go RunBillingOrganisationSync(
			context.Background(),
			billing.NewClient(cfgCopy.Billing),
			orgCopy,
			cfgCopy,
			co.User.Email,
			hostForBilling,
			co.OrgRepo,
		)
	}

	return org, nil
}

// RunBillingOrganisationSync creates the organisation in billing, resolves the license key,
// validates with the license service, encrypts the payload, and updates the org's license data.
// It is called in a goroutine from CreateOrganisationService.Run; it can also be called
// synchronously from tests with a mock billing client and org repo.
func RunBillingOrganisationSync(
	ctx context.Context,
	billingClient billing.Client,
	org datastore.Organisation,
	cfg config.Configuration,
	userEmail string,
	billingHost string,
	orgRepo datastore.OrganisationRepository,
) {
	orgData := billing.BillingOrganisation{
		Name:         org.Name,
		ExternalID:   org.UID,
		BillingEmail: userEmail,
		Host:         billingHost,
	}
	resp, createErr := billingClient.CreateOrganisation(ctx, orgData)
	key := ""
	if createErr == nil && resp != nil {
		key = resp.Data.LicenseKey
	}
	if createErr != nil {
		log.FromContext(ctx).WithError(createErr).Error("create_organisation: CreateOrganisation failed")
	}
	if key == "" && createErr == nil {
		licResp, licErr := billingClient.GetOrganisationLicense(ctx, org.UID)
		if licErr == nil && licResp != nil {
			key = licResp.Data.Key
		}
		if licErr != nil {
			log.FromContext(ctx).WithError(licErr).Error("create_organisation: GetOrganisationLicense failed")
		}
	}
	if key != "" {
		var entitlements map[string]interface{}
		lc := licensesvc.NewClient(licensesvc.Config{
			Host:         cfg.LicenseService.Host,
			ValidatePath: cfg.LicenseService.ValidatePath,
			Timeout:      cfg.LicenseService.Timeout,
			RetryCount:   cfg.LicenseService.RetryCount,
		})
		if data, valErr := lc.ValidateLicense(ctx, key); valErr == nil {
			entitlements, _ = data.GetEntitlementsMap()
		}
		payload := &license.LicenseDataPayload{Key: key, Entitlements: entitlements}
		enc, encErr := license.EncryptLicenseData(org.UID, payload)
		if encErr == nil {
			if updateErr := orgRepo.UpdateOrganisationLicenseData(ctx, org.UID, enc); updateErr != nil {
				log.FromContext(ctx).WithError(updateErr).Error("create_organisation: UpdateOrganisationLicenseData failed")
			}
		} else {
			log.FromContext(ctx).WithError(encErr).Error("create_organisation: EncryptLicenseData failed")
		}
	}
}
