package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
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
}

// RefreshLicenseDataForUser refreshes license_data per org after login (non-blocking).
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
	licClient := licensesvc.NewClient(licensesvc.Config{
		Host:         deps.Cfg.LicenseService.Host,
		ValidatePath: deps.Cfg.LicenseService.ValidatePath,
		Timeout:      deps.Cfg.LicenseService.Timeout,
		RetryCount:   deps.Cfg.LicenseService.RetryCount,
		Logger:       deps.Logger,
	})

	for _, org := range orgs {
		RefreshLicenseDataForOrg(ctx, org, defaultKey, deps, licClient)
	}
}

// RefreshLicenseDataForOrg validates a resolved key and persists encrypted license_data.
func RefreshLicenseDataForOrg(ctx context.Context, org datastore.Organisation, defaultKey string, deps RefreshLicenseDataDeps, licClient *licensesvc.Client) {
	if deps.OrgRepo == nil {
		return
	}
	key := resolveKey(ctx, org, defaultKey, deps.Cfg, deps.BillingClient)
	if key == "" {
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
}

// resolveKey picks the license key used to refresh per-org entitlements after login.
//
// Cloud: ask the billing service for the org's provisioned key. There is no
// instance-wide fallback in cloud, so an empty/missing record returns "".
//
// Self-hosted (licensed instance): the operator-configured CONVOY_LICENSE_KEY is the
// authoritative key for the whole instance. Per-org encrypted LicenseData is only used
// when the operator did not provide an instance key (e.g. multi-tenant self-hosted that
// pulled per-org keys via SelfHostedRegisterEmail).
func resolveKey(ctx context.Context, org datastore.Organisation, defaultKey string, cfg config.Configuration, billingClient billing.Client) string {
	if billingClient != nil && cfg.IsCloud() {
		resp, err := billingClient.GetOrganisationLicense(ctx, org.UID)
		if err == nil && resp != nil && resp.Data.Organisation != nil && strings.TrimSpace(resp.Data.Organisation.LicenseKey) != "" {
			return strings.TrimSpace(resp.Data.Organisation.LicenseKey)
		}
		return ""
	}

	if cfg.IsSelfHosted() && !util.IsStringEmpty(defaultKey) {
		return defaultKey
	}

	if org.LicenseData != "" {
		payload, err := license.DecryptLicenseData(org.UID, org.LicenseData)
		if err == nil && strings.TrimSpace(payload.Key) != "" {
			return strings.TrimSpace(payload.Key)
		}
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

func CheckOrganisationProjectLimit(ctx context.Context, org *datastore.Organisation, deps OrgProjectLimitDeps) (bool, error) {
	if deps.Cfg.IsCloud() {
		if deps.BillingClient == nil {
			return false, errors.New("billing client required for organisation project limit on managed cloud")
		}
		resp, err := deps.BillingClient.GetOrganisationLicense(ctx, org.UID)
		if err != nil {
			return false, fmt.Errorf("get organisation licence: %w", err)
		}
		if resp == nil || resp.Data.Organisation == nil || strings.TrimSpace(resp.Data.Organisation.LicenseKey) == "" {
			return false, errors.New("organisation licence is not provisioned in billing yet")
		}
		return organisationProjectLimitFromLicenseKey(ctx, org, strings.TrimSpace(resp.Data.Organisation.LicenseKey), deps)
	}
	defaultKey := deps.Cfg.LicenseKey
	key := resolveKey(ctx, *org, defaultKey, deps.Cfg, deps.BillingClient)
	if key == "" {
		// No resolvable license key: do not treat as "at project cap" (avoids false 402 on self-hosted).
		return true, nil
	}
	return organisationProjectLimitFromLicenseKey(ctx, org, key, deps)
}

func organisationProjectLimitFromLicenseKey(ctx context.Context, org *datastore.Organisation, key string, deps OrgProjectLimitDeps) (bool, error) {
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
