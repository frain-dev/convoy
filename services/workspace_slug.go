package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/pkg/log"
)

// ResolveWorkspaceBySlugDeps holds dependencies for resolving workspace by slug and syncing Convoy org.
type ResolveWorkspaceBySlugDeps struct {
	BillingClient billing.Client
	OrgRepo       datastore.OrganisationRepository
	Logger        log.StdLogger
	Cfg           config.Configuration
	// RefreshDeps is used to call RefreshLicenseDataForOrg; can reuse same deps as auth handlers.
	RefreshDeps RefreshLicenseDataDeps
}

// ResolveWorkspaceBySlugResult is the result of resolving a workspace by slug from Overwatch and syncing Convoy.
type ResolveWorkspaceBySlugResult struct {
	ExternalID   string
	LicenseKey   string
	SSOAvailable bool
	Org          *datastore.Organisation
}

// ResolveWorkspaceBySlug calls Overwatch workspace_config by slug, then loads Convoy org by external_id (UID),
// refreshes license_data for that org, and returns the OW payload plus the reloaded org.
// Returns error if slug is empty, OW returns error/404, or Convoy org not found.
func ResolveWorkspaceBySlug(ctx context.Context, slug string, deps ResolveWorkspaceBySlugDeps) (*ResolveWorkspaceBySlugResult, error) {
	if slug == "" {
		return nil, errors.New("slug is required")
	}
	if deps.BillingClient == nil {
		return nil, errors.New("billing client is required")
	}
	if deps.OrgRepo == nil {
		return nil, errors.New("org repo is required")
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := deps.BillingClient.GetWorkspaceConfigBySlug(reqCtx, slug)
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.WithError(err).WithField("slug", slug).Debug("workspace_config by slug failed")
		}
		return nil, fmt.Errorf("workspace not found: %w", err)
	}
	if !resp.Status {
		return nil, errors.New("workspace not found")
	}
	if resp.Data.ExternalID == "" {
		return nil, errors.New("workspace config missing external_id: ensure the organisation in Overwatch has external_id set to the Convoy organisation UID")
	}

	org, err := deps.OrgRepo.FetchOrganisationByID(ctx, resp.Data.ExternalID)
	if err != nil {
		return nil, fmt.Errorf("organisation not found for workspace: %w", err)
	}

	// Refresh Convoy org license_data so local state stays in sync.
	defaultKey := deps.Cfg.LicenseKey
	billingEnabled := deps.Cfg.Billing.Enabled && deps.RefreshDeps.BillingClient != nil
	licClient := licensesvc.NewClient(licensesvc.Config{
		Host:         deps.Cfg.LicenseService.Host,
		ValidatePath: deps.Cfg.LicenseService.ValidatePath,
		Timeout:      deps.Cfg.LicenseService.Timeout,
		RetryCount:   deps.Cfg.LicenseService.RetryCount,
		Logger:       deps.Logger,
	})
	RefreshLicenseDataForOrg(ctx, *org, defaultKey, billingEnabled, deps.RefreshDeps, licClient)

	// Re-load org after refresh.
	org, err = deps.OrgRepo.FetchOrganisationByID(ctx, resp.Data.ExternalID)
	if err != nil {
		// Return the result we have; refresh may have updated anyway.
		org = nil
	}

	return &ResolveWorkspaceBySlugResult{
		ExternalID:   resp.Data.ExternalID,
		LicenseKey:   resp.Data.LicenseKey,
		SSOAvailable: resp.Data.SSOAvailable,
		Org:          org,
	}, nil
}
