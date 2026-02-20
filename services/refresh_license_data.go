package services

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

// RefreshLicenseDataDeps holds dependencies for refreshing license data per org.
type RefreshLicenseDataDeps struct {
	OrgMemberRepo datastore.OrganisationMemberRepository
	OrgRepo       datastore.OrganisationRepository
	BillingClient billing.Client
	Logger        log.StdLogger
	Cfg           config.Configuration
}

// RefreshLicenseDataForUser loads the user's organisations and asynchronously refreshes
// license_data (key + entitlements) for each org. Use in a goroutine after login; it uses
// context.Background() and does not block the request.
// Key resolution: existing org license_data (decrypted) else billing GetOrganisationLicense else default (instance) license.
func RefreshLicenseDataForUser(userID string, deps RefreshLicenseDataDeps) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if deps.Logger == nil {
		return
	}
	logger := deps.Logger.WithFields(log.Fields{"user_id": userID})

	// Use first-page cursor: empty cursor would make the query use o.id <= '' which matches no ULID org ids.
	orgs, _, err := deps.OrgMemberRepo.LoadUserOrganisationsPaged(ctx, userID, datastore.Pageable{
		PerPage:    100,
		Direction:  datastore.Next,
		NextCursor: "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
	})
	if err != nil {
		logger.WithError(err).Warn("refresh license data: failed to load user organisations")
		return
	}
	if len(orgs) == 0 {
		return
	}

	defaultKey := deps.Cfg.LicenseKey
	billingEnabled := deps.Cfg.Billing.Enabled && deps.BillingClient != nil
	licClient := licensesvc.NewClient(licensesvc.Config{
		Host:         deps.Cfg.LicenseService.Host,
		ValidatePath: deps.Cfg.LicenseService.ValidatePath,
		Timeout:      deps.Cfg.LicenseService.Timeout,
		RetryCount:   deps.Cfg.LicenseService.RetryCount,
		Logger:       deps.Logger,
	})

	for _, org := range orgs {
		RefreshLicenseDataForOrg(ctx, org, defaultKey, billingEnabled, deps, licClient)
	}
}

// RefreshLicenseDataForOrg resolves key for the org (from billing when enabled, else default), validates, encrypts, and updates org license_data.
// Caller must pass a non-nil licClient.
func RefreshLicenseDataForOrg(ctx context.Context, org datastore.Organisation, defaultKey string, billingEnabled bool, deps RefreshLicenseDataDeps, licClient *licensesvc.Client) {
	if deps.OrgRepo == nil {
		return
	}
	key := resolveKey(ctx, org, defaultKey, billingEnabled, deps.BillingClient)
	if key == "" {
		if billingEnabled {
			if err := deps.OrgRepo.UpdateOrganisationLicenseData(ctx, org.UID, ""); err != nil {
				if deps.Logger != nil {
					deps.Logger.WithError(err).WithField("org_id", org.UID).Warn("refresh license data: clear license_data failed")
				}
			}
		}
		return
	}
	data, err := licClient.ValidateLicense(ctx, key)
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.WithError(err).WithField("org_id", org.UID).Warn("refresh license data: validate failed")
		}
		return
	}
	entitlements, err := data.GetEntitlementsMap()
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.WithError(err).WithField("org_id", org.UID).Warn("refresh license data: get entitlements failed")
		}
		return
	}
	payload := &license.LicenseDataPayload{Key: key, Entitlements: entitlements}
	enc, err := license.EncryptLicenseData(org.UID, payload)
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.WithError(err).WithField("org_id", org.UID).Warn("refresh license data: encrypt failed")
		}
		return
	}
	if err := deps.OrgRepo.UpdateOrganisationLicenseData(ctx, org.UID, enc); err != nil {
		if deps.Logger != nil {
			deps.Logger.WithError(err).WithField("org_id", org.UID).Warn("refresh license data: update failed")
		}
		return
	}
}

// resolveKey returns the org's license key: from billing when enabled, else default. Always resolves from billing when enabled so refresh repopulates license_data.
func resolveKey(ctx context.Context, org datastore.Organisation, defaultKey string, billingEnabled bool, billingClient billing.Client) string {
	if billingEnabled && billingClient != nil {
		resp, err := billingClient.GetOrganisationLicense(ctx, org.UID)
		if err == nil && resp != nil && resp.Data.Organisation != nil && resp.Data.Organisation.LicenseKey != "" {
			return resp.Data.Organisation.LicenseKey
		}
	}

	if billingEnabled {
		return ""
	}
	if !util.IsStringEmpty(defaultKey) {
		return defaultKey
	}
	return ""
}

// OrgProjectLimitDeps holds dependencies for checking org-scoped project limit (billing mode).
type OrgProjectLimitDeps struct {
	BillingClient billing.Client
	ProjectRepo   datastore.ProjectRepository
	Cfg           config.Configuration
	Logger        log.StdLogger
}

// CheckOrganisationProjectLimit returns whether the org is allowed to create another project
// based on its license (org license_data or billing GetOrganisationLicense when billing enabled).
// Call only when billing is enabled.
func CheckOrganisationProjectLimit(ctx context.Context, org *datastore.Organisation, deps OrgProjectLimitDeps) (bool, error) {
	defaultKey := deps.Cfg.LicenseKey
	billingEnabled := deps.Cfg.Billing.Enabled && deps.BillingClient != nil
	key := resolveKey(ctx, *org, defaultKey, billingEnabled, deps.BillingClient)
	if key == "" {
		return false, nil
	}
	licClient := licensesvc.NewClient(licensesvc.Config{
		Host:         deps.Cfg.LicenseService.Host,
		ValidatePath: deps.Cfg.LicenseService.ValidatePath,
		Timeout:      deps.Cfg.LicenseService.Timeout,
		RetryCount:   deps.Cfg.LicenseService.RetryCount,
		Logger:       deps.Logger,
	})
	data, err := licClient.ValidateLicense(ctx, key)
	if err != nil {
		return false, err
	}
	entitlementsMap, err := data.GetEntitlementsMap()
	if err != nil {
		return false, err
	}
	entitlements := licensesvc.ParseEntitlements(entitlementsMap)
	limit, exists := licensesvc.GetNumberEntitlement(entitlements, "project_limit")
	if !exists {
		return false, nil
	}
	if limit == -1 {
		return true, nil
	}
	projects, err := deps.ProjectRepo.LoadProjects(ctx, &datastore.ProjectFilter{OrgID: org.UID})
	if err != nil {
		return false, err
	}
	return int64(len(projects)) < limit, nil
}
