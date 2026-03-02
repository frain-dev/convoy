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

// ResolveWorkspaceBySlugDeps holds dependencies for ResolveWorkspaceBySlug.
type ResolveWorkspaceBySlugDeps struct {
	BillingClient billing.Client
	OrgRepo       datastore.OrganisationRepository
	Logger        log.StdLogger
	Cfg           config.Configuration
	RefreshDeps   RefreshLicenseDataDeps
}

// ResolveWorkspaceBySlugResult is the result of ResolveWorkspaceBySlug.
type ResolveWorkspaceBySlugResult struct {
	ExternalID   string
	LicenseKey   string
	SSOAvailable bool
	Org          *datastore.Organisation
}

// ResolveWorkspaceBySlug resolves workspace by slug via billing and syncs license data for the org.
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
		return nil, errors.New("workspace config missing external_id")
	}

	org, err := deps.OrgRepo.FetchOrganisationByID(ctx, resp.Data.ExternalID)
	if err != nil {
		return nil, fmt.Errorf("organisation not found for workspace: %w", err)
	}

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

	org, err = deps.OrgRepo.FetchOrganisationByID(ctx, resp.Data.ExternalID)
	if err != nil {
		org = nil
	}

	return &ResolveWorkspaceBySlugResult{
		ExternalID:   resp.Data.ExternalID,
		LicenseKey:   resp.Data.LicenseKey,
		SSOAvailable: resp.Data.SSOAvailable,
		Org:          org,
	}, nil
}
