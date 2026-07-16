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
	"github.com/frain-dev/convoy/pkg/logger"
)

// ResolveWorkspaceBySlugDeps holds dependencies for workspace slug resolution.
type ResolveWorkspaceBySlugDeps struct {
	BillingClient billing.Client
	OrgRepo       datastore.OrganisationRepository
	Logger        logger.Logger
	Cfg           config.Configuration
	RefreshDeps   RefreshLicenseDataDeps
}

// ResolveWorkspaceBySlugResult is the result of workspace slug resolution.
type ResolveWorkspaceBySlugResult struct {
	ExternalID   string
	LicenseKey   string
	SSOAvailable bool
	Org          *datastore.Organisation
}

// LookupWorkspaceBySlug resolves a workspace by slug without license refresh side effects.
// Failure policy: fail closed. Guest routes must not trigger billing/license writes on read.
func LookupWorkspaceBySlug(ctx context.Context, slug string, deps ResolveWorkspaceBySlugDeps) (*ResolveWorkspaceBySlugResult, error) {
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
			deps.Logger.Debug("workspace_config by slug failed", "error", err, "slug", slug)
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

	return &ResolveWorkspaceBySlugResult{
		ExternalID:   resp.Data.ExternalID,
		LicenseKey:   resp.Data.LicenseKey,
		SSOAvailable: resp.Data.SSOAvailable,
		Org:          org,
	}, nil
}

// ResolveWorkspaceBySlug resolves workspace by slug and syncs license data for the org.
// Use only on authenticated paths that intentionally refresh license state.
func ResolveWorkspaceBySlug(ctx context.Context, slug string, deps ResolveWorkspaceBySlugDeps) (*ResolveWorkspaceBySlugResult, error) {
	result, err := LookupWorkspaceBySlug(ctx, slug, deps)
	if err != nil {
		return nil, err
	}

	defaultKey := deps.Cfg.LicenseKey
	useOrgBilling := deps.Cfg.UsesOrgBilling() && deps.RefreshDeps.BillingClient != nil
	licClient := licensesvc.NewClient(licensesvc.Config{
		Host:         deps.Cfg.LicenseService.Host,
		ValidatePath: deps.Cfg.LicenseService.ValidatePath,
		Timeout:      deps.Cfg.LicenseService.Timeout,
		RetryCount:   deps.Cfg.LicenseService.RetryCount,
		Logger:       deps.Logger,
	})
	RefreshLicenseDataForOrg(ctx, *result.Org, defaultKey, useOrgBilling, deps.RefreshDeps, licClient)

	org, err := deps.OrgRepo.FetchOrganisationByID(ctx, result.ExternalID)
	if err != nil {
		result.Org = nil
	} else {
		result.Org = org
	}

	return result, nil
}
