package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore/cached"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/pkg/cachedrepo"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

// usageLockReleaseScript releases the recompute lock only if the caller still
// owns it (atomic compare-and-delete) so a worker that outlived the TTL cannot
// clear a lock a newer worker has since acquired.
var usageLockReleaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
end
return 0
`)

func (h *BillingHandler) ensureOrganisationInBilling(w http.ResponseWriter, r *http.Request, orgID string) bool {
	orgRepo := h.orgRepo()
	org, err := orgRepo.FetchOrganisationByID(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to fetch organisation data", http.StatusInternalServerError))
		return true
	}

	cfg, err := config.Get()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to fetch config", http.StatusInternalServerError))
		return true
	}

	if cfg.Host == "" {
		_ = render.Render(w, r, util.NewErrorResponse(ErrHostRequiredForBilling.Error(), http.StatusBadRequest))
		return true
	}

	ownerEmail := h.getOwnerEmail(r.Context(), orgID)
	if ownerEmail == "" {
		_ = render.Render(w, r, util.NewErrorResponse(ErrOwnerEmailRequiredForBilling.Error(), http.StatusUnprocessableEntity))
		return true
	}

	orgData := billing.BillingOrganisation{
		Name:         org.Name,
		ExternalID:   orgID,
		BillingEmail: ownerEmail,
		Host:         cfg.Host,
	}

	_, createErr := h.BillingClient.CreateOrganisation(r.Context(), orgData)
	if createErr != nil {
		errorMsg := createErr.Error()
		if strings.Contains(errorMsg, "Validation failed") {
			_ = render.Render(w, r, util.NewErrorResponse(errorMsg, http.StatusBadRequest))
		} else {
			_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("Failed to create organisation in billing service: %s", errorMsg), http.StatusInternalServerError))
		}
		return true
	}
	return false
}

// getOrCreateBillingOrg fetches the billing organisation, creating it on first use when
// the billing service reports it missing and then refetching. On any rendered error it
// returns ok=false and the caller must return.
func (h *BillingHandler) getOrCreateBillingOrg(w http.ResponseWriter, r *http.Request, orgID string) (*billing.Response[billing.BillingOrganisation], bool) {
	resp, err := h.BillingClient.GetOrganisation(r.Context(), orgID)
	if err != nil && isBillingOrgNotFound(err) {
		if h.ensureOrganisationInBilling(w, r, orgID) {
			return nil, false
		}
		resp, err = h.BillingClient.GetOrganisation(r.Context(), orgID)
	}
	if err != nil {
		renderBillingError(w, r, err)
		return nil, false
	}
	return resp, true
}

// renderBillingError renders a billing service failure as a 500. Endpoints that
// intentionally map the same failure to a different status (e.g. GetUsage returns 503)
// keep their own rendering instead of calling this.
func renderBillingError(w http.ResponseWriter, r *http.Request, err error) {
	_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
}

// updateBillingEmailIfEmpty backfills the organisation's billing email when the billing
// service has none. It is best-effort and fire-and-forget: it runs in its own goroutine
// with a background context, and the failure policy is fail-open (errors are only logged,
// never surfaced to the caller).
func (h *BillingHandler) updateBillingEmailIfEmpty(orgID string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := h.BillingClient.GetOrganisation(bgCtx, orgID)
		if err != nil {
			return
		}

		if resp.Data.BillingEmail != "" {
			return
		}

		ownerEmail := h.getOwnerEmail(bgCtx, orgID)
		if ownerEmail == "" {
			return
		}

		updateData := billing.BillingOrganisation{
			BillingEmail: ownerEmail,
		}
		_, updateErr := h.BillingClient.UpdateOrganisation(bgCtx, orgID, updateData)
		if updateErr != nil {
			h.A.Logger.Warnf("Failed to update billing_email for organisation %s: %v", orgID, updateErr)
		} else {
			h.A.Logger.Infof("Updated billing_email for organisation %s", orgID)
		}
	}()
}

const (
	// usageCacheTTL is how long a computed usage figure is served before a
	// background refresh recomputes it.
	usageCacheTTL = time.Hour
	// usageRecomputeLockTTL bounds a single background recompute and dedupes
	// concurrent recomputes for the same org/period.
	usageRecomputeLockTTL = 2 * time.Minute
)

// GetUsage returns the org's usage. Behaviour depends on the configured source:
//
//   - billing-service: the provider owns the figure and the billing period, so
//     serve it synchronously and surface provider failures as 503. Caching or a
//     pending placeholder would hide an outage behind a state the caller could
//     never escape.
//   - postgres (default): aggregate from this instance's persisted byte columns
//     without blocking the caller. Serve the last computed value from Redis and
//     refresh it in the background (stale-while-revalidate, like dashboard
//     stats). On a cold cache return a pending response so the dashboard renders
//     a placeholder instead of a misleading zero until the real figure is known.
func (h *BillingHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	cfg, err := config.Get()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to fetch config", http.StatusInternalServerError))
		return
	}

	// Billing-service source: synchronous, no cache. A provider failure must
	// reach the caller as 503 rather than an indefinite pending placeholder.
	if cfg.Billing.UsageSource == config.BillingUsageSourceBillingService {
		resp, gErr := h.BillingClient.GetUsage(r.Context(), orgID)
		if gErr != nil {
			_ = render.Render(w, r, util.NewErrorResponse(gErr.Error(), http.StatusServiceUnavailable))
			return
		}
		usage := resp.Data
		usage.Pending = false
		_ = render.Render(w, r, util.NewServerResponse("Usage retrieved successfully", &usage, http.StatusOK))
		return
	}

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)

	// Usage window. Default to the current calendar month when no range is
	// supplied. When the caller does supply a range (the subscription billing
	// cycle the UI displays), reject it if malformed rather than silently
	// aggregating a different window than the figure is labeled with. Cap the
	// span at ~1 year to bound the aggregation.
	startTime, endTime := startOfMonth, endOfMonth
	if s, e := r.URL.Query().Get("start"), r.URL.Query().Get("end"); s != "" || e != "" {
		ps, errS := time.Parse(time.RFC3339, s)
		pe, errE := time.Parse(time.RFC3339, e)
		if errS != nil || errE != nil || !ps.Before(pe) || pe.Sub(ps) > 366*24*time.Hour {
			_ = render.Render(w, r, util.NewErrorResponse("invalid usage window: start and end must be RFC3339 timestamps with start before end and a span of at most 366 days", http.StatusBadRequest))
			return
		}
		startTime, endTime = ps, pe
	}
	period := startTime.Format("2006-01")

	// The Postgres source aggregates over the requested window, so the cache key
	// varies by range.
	cacheKey := fmt.Sprintf("billing:usage:%s:%d-%d", orgID, startTime.UnixNano(), endTime.UnixNano())

	// Fail soft: a cache read error (Redis unavailable) is treated like a miss
	// so the caller is never blocked. The background recompute is itself gated
	// by a Redis lock below that also fails closed when Redis is down, so a
	// read outage cannot stampede Postgres with concurrent aggregations.
	var cached *billing.Usage
	if cacheErr := h.A.Cache.Get(r.Context(), cacheKey, &cached); cacheErr != nil {
		h.A.Logger.Error("failed to read usage from cache", "error", cacheErr)
	}

	if cached != nil {
		h.recomputeUsageInBackground(orgID, period, cacheKey, startTime, endTime)
		_ = render.Render(w, r, util.NewServerResponse("Usage retrieved successfully", cached, http.StatusOK))
		return
	}

	h.recomputeUsageInBackground(orgID, period, cacheKey, startTime, endTime)

	pending := &billing.Usage{
		OrganisationID: orgID,
		Period:         period,
		Pending:        true,
		CreatedAt:      now.Format(time.RFC3339),
	}
	_ = render.Render(w, r, util.NewServerResponse("Usage is being calculated", pending, http.StatusOK))
}

// recomputeUsageInBackground refreshes the cached usage figure off the request
// path. An atomic Redis lock dedupes concurrent recomputes per org so a burst of
// requests cannot stampede Postgres.
func (h *BillingHandler) recomputeUsageInBackground(orgID, period, cacheKey string, startTime, endTime time.Time) {
	// Fail closed: without Redis we cannot dedupe, so skip the recompute rather
	// than risk concurrent heavy aggregations. The caller still gets a pending
	// response and the figure resolves once Redis is available again.
	if h.A.Redis == nil {
		h.A.Logger.Error("skipping usage recompute: redis is not configured")
		return
	}

	// Scope the lock to the org, not the requested window: a billing-authorized
	// caller could otherwise bypass dedup by issuing many distinct windows, each
	// spawning its own heavy aggregation. At most one recompute runs per org at a
	// time; the result is still cached under the window-specific cacheKey.
	lockKey := fmt.Sprintf("billing:usage:%s:query", orgID)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), usageRecomputeLockTTL)
		defer cancel()

		// Atomic acquire. Fail closed on Redis error (skip rather than run a
		// duplicate aggregation); skip quietly if another recompute holds it.
		token := ulid.Make().String()
		acquired, err := h.A.Redis.SetNX(ctx, lockKey, token, usageRecomputeLockTTL).Result()
		if err != nil {
			h.A.Logger.Error("failed to acquire usage recompute lock", "error", err)
			return
		}
		if !acquired {
			h.A.Logger.Debug("usage recompute already running")
			return
		}
		defer h.releaseUsageLock(lockKey, token)

		// Fail-to-placeholder: if the aggregation errors or exceeds the lock TTL
		// (e.g. a large window still falling back to payload scans for
		// pre-migration rows), nothing is cached and the caller keeps seeing the
		// pending sentinel. The dashboard polls a bounded number of times and
		// then renders a placeholder. The figure resolves on a later recompute as
		// more rows carry the persisted byte columns and the read goes index-only.
		usage, err := h.computeUsage(ctx, orgID, period, startTime, endTime)
		if err != nil {
			h.A.Logger.Error("failed to compute usage", "error", err)
			return
		}

		// Persist with a fresh short-lived context: a compute that consumed most
		// of the lock budget would otherwise fail Cache.Set on the already-expired
		// context, discarding a successful result and leaving callers stuck on the
		// pending sentinel.
		setCtx, cancelSet := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelSet()
		if err := h.A.Cache.Set(setCtx, cacheKey, usage, usageCacheTTL); err != nil {
			h.A.Logger.Error("failed to cache usage", "error", err)
		}
	}()
}

// releaseUsageLock releases the recompute lock with an owner check so a worker
// that overran the TTL cannot delete a newer worker's lock. Uses a fresh
// timeout so release still runs if the compute context was cancelled.
func (h *BillingHandler) releaseUsageLock(lockKey, token string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := usageLockReleaseScript.Run(ctx, h.A.Redis, []string{lockKey}, token).Err(); err != nil {
		h.A.Logger.Error("failed to release usage recompute lock", "error", err)
	}
}

// computeUsage aggregates usage from this instance's persisted byte columns for
// the requested window; the column reads converge to index-only as the window
// fills with populated rows. Only the Postgres source uses this background path,
// the billing-service source is served synchronously by GetUsage.
func (h *BillingHandler) computeUsage(ctx context.Context, orgID, period string, startTime, endTime time.Time) (*billing.Usage, error) {
	usage, err := h.orgRepo().CalculateUsage(ctx, orgID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return &billing.Usage{
		OrganisationID: usage.OrganisationID,
		Period:         period,
		Received:       billing.UsageMetrics{Volume: usage.Received.Volume, Bytes: usage.Received.Bytes},
		Sent:           billing.UsageMetrics{Volume: usage.Sent.Volume, Bytes: usage.Sent.Bytes},
		CreatedAt:      usage.CreatedAt.Format(time.RFC3339),
		Pending:        false,
	}, nil
}

func (h *BillingHandler) GetInvoices(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetInvoices(r.Context(), orgID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Invoices retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	// Unlike GetOrganisation/GetInternalOrganisationID, this path only triggers a billing
	// org create on "not found" and otherwise tolerates GetOrganisation errors, so it is
	// not folded into getOrCreateBillingOrg (which would render on any error).
	_, err := h.BillingClient.GetOrganisation(r.Context(), orgID)
	if err != nil && isBillingOrgNotFound(err) {
		if h.ensureOrganisationInBilling(w, r, orgID) {
			return
		}
	}

	resp, err := h.BillingClient.GetSubscription(r.Context(), orgID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	h.updateOrganisationStatus(r.Context(), orgID, billing.HasActiveSubscription(resp.Data))
	h.updateBillingEmailIfEmpty(orgID)

	_ = render.Render(w, r, util.NewServerResponse("Subscription retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetPaymentMethods(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetPaymentMethods(r.Context(), orgID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Payment methods retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) SetDefaultPaymentMethod(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	pmID := chi.URLParam(r, "pmID")
	if orgID == "" || pmID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and payment method ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.SetDefaultPaymentMethod(r.Context(), orgID, pmID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Default payment method set successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) DeletePaymentMethod(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	pmID := chi.URLParam(r, "pmID")
	if orgID == "" || pmID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and payment method ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.DeletePaymentMethod(r.Context(), orgID, pmID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Payment method deleted successfully", resp.Data, http.StatusOK))
}

// Organisation handlers
func (h *BillingHandler) GetOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, ok := h.getOrCreateBillingOrg(w, r, orgID)
	if !ok {
		return
	}

	h.updateBillingEmailIfEmpty(orgID)
	_ = render.Render(w, r, util.NewServerResponse("Organisation retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	var orgData billing.BillingOrganisation
	if err := json.NewDecoder(r.Body).Decode(&orgData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateOrganisation(r.Context(), orgID, orgData)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation updated successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateOrganisationTaxID(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	var taxData billing.UpdateOrganisationTaxIDRequest
	if err := json.NewDecoder(r.Body).Decode(&taxData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateOrganisationTaxID(r.Context(), orgID, taxData)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Tax ID updated successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateOrganisationAddress(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	var addressData billing.UpdateOrganisationAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&addressData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateOrganisationAddress(r.Context(), orgID, addressData)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Address updated successfully", resp.Data, http.StatusOK))
}

// Subscription handlers
func (h *BillingHandler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetSubscriptions(r.Context(), orgID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscriptions retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) OnboardSubscription(w http.ResponseWriter, r *http.Request) {
	// Only require billing access, not enabled organisation - allows onboarding even if org is disabled
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	var requestData billing.OnboardSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	if err := validatePlanAndHostRequired(requestData.PlanID, requestData.Host); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.OnboardSubscription(r.Context(), orgID, requestData)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Checkout session created successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) StartTrial(w http.ResponseWriter, r *http.Request) {
	// Same access gate as OnboardSubscription; trial eligibility is enforced by billing.
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	var requestData billing.StartTrialRequest
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil && err != io.EOF {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.StartTrial(r.Context(), orgID, requestData)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	h.provisionTrialCap(r.Context(), orgID)
	h.activateTrialCap(orgID)

	_ = render.Render(w, r, util.NewServerResponse("Trial started successfully", resp.Data, http.StatusOK))
}

const (
	provisionalTrialDailyEventLimit = 100
	provisionalTrialOrgLimit        = 1
	provisionalTrialUserLimit       = 1
	provisionalTrialProjectLimit    = 1
)

func provisionalTrialEntitlements() map[string]interface{} {
	return map[string]interface{}{
		"daily_event_limit": provisionalTrialDailyEventLimit,
		"org_limit":         provisionalTrialOrgLimit,
		"user_limit":        provisionalTrialUserLimit,
		"project_limit":     provisionalTrialProjectLimit,
	}
}

func (h *BillingHandler) provisionTrialCap(ctx context.Context, orgID string) {
	if !h.A.Cfg.UsesOrgBilling() {
		return
	}

	payload := &license.LicenseDataPayload{
		Provisional:  true,
		Entitlements: provisionalTrialEntitlements(),
	}

	enc, err := license.EncryptLicenseData(orgID, payload)
	if err != nil {
		h.A.Logger.Warn("start trial: encrypt provisional trial seed failed", "error", err, "org_id", orgID)
		return
	}

	if err := h.orgRepo().UpdateOrganisationLicenseData(ctx, orgID, enc); err != nil {
		h.A.Logger.Warn("start trial: write provisional trial seed failed", "error", err, "org_id", orgID)
		return
	}

	if h.A.Cache != nil {
		cachedrepo.Invalidate(ctx, h.A.Cache, h.A.Logger, cached.OrganisationCacheKey(orgID))
	}
}

// trial cap activation polling budget: attempts and initial backoff (doubling, capped).
const (
	trialCapActivateAttempts = 6
	trialCapActivateBackoff  = time.Second
)

func (h *BillingHandler) reconcileTrialCapOnce(ctx context.Context, orgID string, useOrgBilling bool, deps services.RefreshLicenseDataDeps, licClient *licensesvc.Client) bool {
	org, err := h.orgRepo().FetchOrganisationByID(ctx, orgID)
	if err != nil || org == nil {
		h.A.Logger.Warn("start trial: fetch org for cap activation failed; retrying", "error", err, "org_id", orgID)
		return false
	}

	wasProvisional := license.IsProvisional(org.UID, org.LicenseData)

	services.RefreshLicenseDataForOrg(ctx, *org, h.A.Cfg.LicenseKey, useOrgBilling, deps, licClient)

	refreshed, rerr := h.orgRepo().FetchOrganisationByID(ctx, orgID)
	if rerr != nil || refreshed == nil {
		return false
	}

	if license.HasAuthoritativeEntitlements(refreshed.UID, refreshed.LicenseData) {
		if license.HasDailyEventLimitEntitlement(refreshed.UID, refreshed.LicenseData) {
			h.A.Logger.Info("start trial: authoritative entitlements active", "org_id", orgID, "daily_event_limit", license.DailyEventLimit(refreshed.UID, refreshed.LicenseData))
			return true
		}
		if wasProvisional {
			// Trial activation: a non-provisional refresh without daily_event_limit is
			// billing fail-open or stale; keep polling and re-seed provisional caps.
			h.A.Logger.Warn("start trial: rejected authoritative payload without daily_event_limit (billing fail-open or stale); re-seeding provisional caps", "org_id", orgID)
		} else {
			// Paid or other final authoritative payloads often omit daily_event_limit.
			return true
		}
	}

	if !license.IsProvisional(refreshed.UID, refreshed.LicenseData) {
		h.provisionTrialCap(ctx, orgID)
	}
	return false
}

func (h *BillingHandler) activateTrialCap(orgID string) {
	useOrgBilling := h.A.Cfg.UsesOrgBilling() && h.A.BillingClient != nil
	if !useOrgBilling {
		return
	}

	go func() {
		licClient := licensesvc.NewClientFromConfig(h.A.Cfg.LicenseService, h.A.Logger)
		deps := services.RefreshLicenseDataDeps{
			OrgRepo:       h.orgRepo(),
			BillingClient: h.A.BillingClient,
			Logger:        h.A.Logger,
			Cfg:           h.A.Cfg,
			Cache:         h.A.Cache,
		}

		backoff := trialCapActivateBackoff
		for attempt := 0; attempt < trialCapActivateAttempts; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			done := h.reconcileTrialCapOnce(ctx, orgID, useOrgBilling, deps, licClient)
			cancel()
			if done {
				return
			}

			time.Sleep(backoff)
			if backoff < 8*time.Second {
				backoff *= 2
			}
		}
		h.A.Logger.Warn("start trial: authoritative entitlements not active after polling; provisional cap remains, relying on async license refresh", "org_id", orgID)
	}()
}

func (h *BillingHandler) UpgradeSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	subscriptionID := chi.URLParam(r, "subscriptionID")
	if orgID == "" || subscriptionID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and subscription ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	var requestData billing.UpgradeSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	if err := validatePlanAndHostRequired(requestData.PlanID, requestData.Host); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpgradeSubscription(r.Context(), orgID, subscriptionID, requestData)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Checkout session created successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	subscriptionID := chi.URLParam(r, "subscriptionID")
	if orgID == "" || subscriptionID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and subscription ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.DeleteSubscription(r.Context(), orgID, subscriptionID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	// A cancelled subscription is always inactive; resp.Data here is an opaque payload
	// (Response[interface{}]) that never carried active state, so pass active=false.
	h.updateOrganisationStatus(r.Context(), orgID, false)

	_ = render.Render(w, r, util.NewServerResponse("Subscription cancelled successfully", resp.Data, http.StatusOK))
}

// Payment method handlers
func (h *BillingHandler) GetSetupIntent(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetSetupIntent(r.Context(), orgID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Setup intent retrieved successfully", resp.Data, http.StatusOK))
}

// Invoice handlers
func (h *BillingHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	invoiceID := chi.URLParam(r, "invoiceID")
	if orgID == "" || invoiceID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and invoice ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.GetInvoice(r.Context(), orgID, invoiceID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Invoice retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) DownloadInvoice(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	invoiceID := chi.URLParam(r, "invoiceID")
	if orgID == "" || invoiceID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and invoice ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	pdfResp, err := h.BillingClient.DownloadInvoice(r.Context(), orgID, invoiceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			_ = render.Render(w, r, util.NewErrorResponse("Invoice not found", http.StatusNotFound))
		} else if strings.Contains(err.Error(), "PDF link not found") {
			_ = render.Render(w, r, util.NewErrorResponse("Invoice PDF link not available", http.StatusNotFound))
		} else {
			_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("Failed to download invoice: %s", err.Error()), http.StatusInternalServerError))
		}
		return
	}
	defer pdfResp.Body.Close()

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="invoice-%s.pdf"`, invoiceID))

	_, err = io.Copy(w, pdfResp.Body)
	if err != nil {
		h.A.Logger.Error("Failed to stream PDF to client", "error", err)
		return
	}
}

// GetInternalOrganisationID returns the internal organisation ID from billing service
func (h *BillingHandler) GetInternalOrganisationID(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, ok := h.getOrCreateBillingOrg(w, r, orgID)
	if !ok {
		return
	}

	h.updateBillingEmailIfEmpty(orgID)

	responseData := map[string]interface{}{
		"id": resp.Data.ID,
	}

	_ = render.Render(w, r, util.NewServerResponse("Internal organisation ID retrieved successfully", responseData, http.StatusOK))
}

func (h *BillingHandler) updateOrganisationStatus(ctx context.Context, orgID string, active bool) {
	orgRepo := h.orgRepo()
	org, err := orgRepo.FetchOrganisationByID(ctx, orgID)
	if err != nil {
		h.A.Logger.Errorf("Failed to fetch organisation %s for disabled status update: %v", orgID, err)
		return
	}

	if !billing.ApplySubscriptionStatus(org, active) {
		return
	}

	if err := orgRepo.UpdateOrganisation(ctx, org); err != nil {
		if active {
			h.A.Logger.Errorf("Failed to clear organisation %s disabled_at: %v", orgID, err)
		} else {
			h.A.Logger.Errorf("Failed to set organisation %s disabled_at: %v", orgID, err)
		}
		return
	}
	if active {
		h.A.Logger.Infof("Cleared organisation %s disabled_at - subscription active", orgID)
	} else {
		h.A.Logger.Infof("Set organisation %s disabled_at - subscription not active", orgID)
	}
}
