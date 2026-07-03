package services

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/cached"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/pkg/cachedrepo"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

// RefreshLicenseDataDeps holds dependencies for refreshing license data per org.
type RefreshLicenseDataDeps struct {
	OrgMemberRepo datastore.OrganisationMemberRepository
	OrgRepo       datastore.OrganisationRepository
	BillingClient billing.Client
	Logger        log.Logger
	Cfg           config.Configuration
	Cache         cachedrepo.Cache
}

// invalidateOrgCache clears the cached organisation entry after a license_data write, so
// the raw-repo write path stays in sync with the cached read path. No-op without a cache.
func invalidateOrgCache(deps RefreshLicenseDataDeps, orgID string) {
	if deps.Cache == nil || deps.Logger == nil {
		return
	}
	cachedrepo.Invalidate(context.Background(), deps.Cache, deps.Logger, cached.OrganisationCacheKey(orgID))
}

// RefreshLicenseDataForUser loads the user's organisations and asynchronously refreshes
// license_data (key + entitlements) for each org. Use in a goroutine after login; it uses
// context.Background() and does not block the request.
// Key resolution: cloud org billing when configured, otherwise default instance license.
func RefreshLicenseDataForUser(userID string, deps RefreshLicenseDataDeps) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if deps.Logger == nil {
		return
	}
	logger := deps.Logger

	// Use first-page cursor: empty cursor would make the query use o.id <= '' which matches no ULID org ids.
	orgs, _, err := deps.OrgMemberRepo.LoadUserOrganisationsPaged(ctx, userID, datastore.Pageable{
		PerPage:    100,
		Direction:  datastore.Next,
		NextCursor: "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
	})
	if err != nil {
		logger.Warn("refresh license data: failed to load user organisations", "error", err, "user_id", userID)
		return
	}
	if len(orgs) == 0 {
		return
	}

	defaultKey := deps.Cfg.LicenseKey
	useOrgBilling := deps.Cfg.UsesOrgBilling() && deps.BillingClient != nil
	licClient := licensesvc.NewClientFromConfig(deps.Cfg.LicenseService, deps.Logger)

	for _, org := range orgs {
		RefreshLicenseDataForOrg(ctx, org, defaultKey, useOrgBilling, deps, licClient)
	}
}

// ClearOrgLicenseData clears license_data unless the org holds a marked provisional trial seed.
func ClearOrgLicenseData(ctx context.Context, deps RefreshLicenseDataDeps, orgID string) error {
	if deps.OrgRepo == nil {
		return nil
	}
	org, err := deps.OrgRepo.FetchOrganisationByID(ctx, orgID)
	if err != nil {
		return err
	}
	if org == nil || org.LicenseData == "" {
		return nil
	}
	if license.IsProvisional(orgID, org.LicenseData) {
		if deps.Logger != nil {
			deps.Logger.Info("license_data clear skipped: preserving provisional trial seed", "org_id", orgID)
		}
		return nil
	}
	if err := deps.OrgRepo.UpdateOrganisationLicenseData(ctx, orgID, ""); err != nil {
		return err
	}
	invalidateOrgCache(deps, orgID)
	return nil
}

// RefreshLicenseDataForOrg resolves key for the org, validates, encrypts, and updates org license_data.
// Caller must pass a non-nil licClient.
func RefreshLicenseDataForOrg(ctx context.Context, org datastore.Organisation, defaultKey string, useOrgBilling bool, deps RefreshLicenseDataDeps, licClient *licensesvc.Client) {
	if deps.OrgRepo == nil {
		return
	}
	key := resolveKey(ctx, org, defaultKey, useOrgBilling, deps.BillingClient)
	if key == "" {
		if useOrgBilling {
			if err := ClearOrgLicenseData(ctx, deps, org.UID); err != nil && deps.Logger != nil {
				deps.Logger.Warn("refresh license data: clear license_data failed", "error", err, "org_id", org.UID)
			}
		}
		return
	}
	data, err := licClient.ValidateLicense(ctx, key)
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.Warn("refresh license data: validate failed", "error", err, "org_id", org.UID)
		}
		return
	}
	entitlements, err := data.GetEntitlementsMap()
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.Warn("refresh license data: get entitlements failed", "error", err, "org_id", org.UID)
		}
		return
	}
	if !license.EntitlementsHaveDailyEventLimit(entitlements) {
		if current, ferr := deps.OrgRepo.FetchOrganisationByID(ctx, org.UID); ferr == nil && current != nil &&
			license.IsProvisional(current.UID, current.LicenseData) {
			if deps.Logger != nil {
				deps.Logger.Info("refresh license data: preserving provisional trial cap; refresh payload lacks daily_event_limit", "org_id", org.UID)
			}
			return
		}
	}

	payload := &license.LicenseDataPayload{Key: key, Entitlements: entitlements}
	enc, err := license.EncryptLicenseData(org.UID, payload)
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.Warn("refresh license data: encrypt failed", "error", err, "org_id", org.UID)
		}
		return
	}
	if err := deps.OrgRepo.UpdateOrganisationLicenseData(ctx, org.UID, enc); err != nil {
		if deps.Logger != nil {
			deps.Logger.Warn("refresh license data: update failed", "error", err, "org_id", org.UID)
		}
		return
	}
	invalidateOrgCache(deps, org.UID)
}

// resolveKey returns the org's license key from cloud org billing when configured, otherwise the default instance key.
func resolveKey(ctx context.Context, org datastore.Organisation, defaultKey string, useOrgBilling bool, billingClient billing.Client) string {
	if useOrgBilling && billingClient != nil {
		resp, err := billingClient.GetOrganisationLicense(ctx, org.UID)
		if err == nil && resp != nil && resp.Data.Organisation != nil && resp.Data.Organisation.LicenseKey != "" {
			return resp.Data.Organisation.LicenseKey
		}
	}

	if useOrgBilling {
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
	Logger        log.Logger
}

// CheckOrganisationProjectLimit returns whether the org is allowed to create another project
// based on its cloud org license_data caps (including provisional trial seeds) or, when no
// finite org-scoped cap applies, its cloud org billing license or the default instance license.
func CheckOrganisationProjectLimit(ctx context.Context, org *datastore.Organisation, deps OrgProjectLimitDeps) (bool, error) {
	if limit, applies := license.OrgEntitlementCap(org.UID, org.LicenseData, "project_limit"); applies {
		projects, err := deps.ProjectRepo.LoadProjects(ctx, &datastore.ProjectFilter{OrgID: org.UID})
		if err != nil {
			return false, err
		}
		return int64(len(projects)) < limit, nil
	}

	defaultKey := deps.Cfg.LicenseKey
	useOrgBilling := deps.Cfg.UsesOrgBilling() && deps.BillingClient != nil
	key := resolveKey(ctx, *org, defaultKey, useOrgBilling, deps.BillingClient)
	if key == "" {
		return false, nil
	}
	licClient := licensesvc.NewClientFromConfig(deps.Cfg.LicenseService, deps.Logger)
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
