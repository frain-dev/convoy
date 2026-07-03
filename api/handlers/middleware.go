package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/cached"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/util"
)

func (h *Handler) RequireEnabledProject() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, err := h.retrieveProject(r)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse("failed to retrieve project", http.StatusBadRequest))
				return
			}

			if err = license.EnsureProjectEnabled(h.A.Licenser, p.UID); err != nil {
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
				return
			}

			ctx := context.WithValue(r.Context(), convoy.ProjectCtx, p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (h *Handler) RequireEnabledOrganisation() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var org *datastore.Organisation
			var err error

			// Try to get project from context (set by RequireEnabledProject middleware)
			var project *datastore.Project
			if cachedProject := r.Context().Value(convoy.ProjectCtx); cachedProject != nil {
				project = cachedProject.(*datastore.Project)
			} else if projectID := chi.URLParam(r, "projectID"); projectID != "" {
				// Fallback: try to fetch project from URL parameter
				project, err = h.retrieveProject(r)
				if err != nil {
					_ = render.Render(w, r, util.NewErrorResponse("failed to retrieve project", http.StatusBadRequest))
					return
				}
			}

			// Fetch organization based on whether we have a project
			if project != nil {
				// We have a project - fetch org from context or by project's organization ID
				if cachedOrg := r.Context().Value(convoy.OrganisationCtx); cachedOrg != nil {
					org = cachedOrg.(*datastore.Organisation)
				} else {
					orgRepo := cached.NewCachedOrganisationRepository(organisations.New(h.A.Logger, h.A.DB), h.A.Cache, 5*time.Minute, h.A.Logger)
					org, err = orgRepo.FetchOrganisationByID(r.Context(), project.OrganisationID)
					if err != nil {
						h.A.Logger.Error("Failed to fetch organisation for disabled check", "error", err)
						_ = render.Render(w, r, util.NewErrorResponse("Failed to verify organization status", http.StatusInternalServerError))
						return
					}
					if org == nil {
						_ = render.Render(w, r, util.NewErrorResponse("Organization not found", http.StatusNotFound))
						return
					}
					ctx := context.WithValue(r.Context(), convoy.OrganisationCtx, org)
					r = r.WithContext(ctx)
				}
			} else {
				// No project - fetch org directly from context or by orgID parameter
				if cachedOrg := r.Context().Value(convoy.OrganisationCtx); cachedOrg != nil {
					org = cachedOrg.(*datastore.Organisation)
				} else {
					org, err = h.retrieveOrganisation(r)
					if err != nil {
						_ = render.Render(w, r, util.NewServiceErrResponse(err))
						return
					}
					if org == nil {
						_ = render.Render(w, r, util.NewErrorResponse("Organization not found", http.StatusNotFound))
						return
					}
					ctx := context.WithValue(r.Context(), convoy.OrganisationCtx, org)
					r = r.WithContext(ctx)
				}
			}

			if org == nil {
				_ = render.Render(w, r, util.NewErrorResponse("Organization not found", http.StatusNotFound))
				return
			}

			if h.A.Cfg.UsesOrgBilling() && h.isOrganisationDisabled(org) {
				_ = render.Render(w, r, util.NewErrorResponse("This action is disabled for this organization. Please contact support or subscribe to a plan.", http.StatusForbidden))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// trialCapDuplicateVerdict resolves whether a keyed incoming event will be treated as
// a duplicate by the create path the cap gate guards. Invariant: gate predicate ==
// service predicate, per path. Each handler must pass the predicate matching how its
// own service/worker decides novelty, otherwise the gate can skip the cap for an event
// the service still processes as new (an uncounted event) or vice versa.
type trialCapDuplicateVerdict func(ctx context.Context, projectID, idempotencyKey string) (bool, error)

// duplicateByAnyEvent mirrors the async create workers (endpoint, broadcast, dynamic
// paths; see worker/task/process_*_event_creation.go): FindEventsByIdempotencyKey
// treats ANY prior event with the key, including rows flagged is_duplicate_event, as
// a duplicate.
func (h *Handler) duplicateByAnyEvent(ctx context.Context, projectID, idempotencyKey string) (bool, error) {
	return h.eventRepo().FindEventsByIdempotencyKey(ctx, projectID, idempotencyKey)
}

// duplicateByFirstNonDuplicateEvent mirrors CreateFanoutEventService:
// FindFirstEventWithIdempotencyKey only matches rows with is_duplicate_event=false, so
// a key that has only ever produced duplicate-flagged rows is treated as NEW — the
// fanout service will enqueue a new non-duplicate event, which must consume quota.
func (h *Handler) duplicateByFirstNonDuplicateEvent(ctx context.Context, projectID, idempotencyKey string) (bool, error) {
	_, err := h.eventRepo().FindFirstEventWithIdempotencyKey(ctx, projectID, idempotencyKey)
	if err != nil {
		if errors.Is(err, datastore.ErrEventNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// enforceTrialEventCapForNewEvent applies the trial event cap only when the incoming
// event would be processed as new. Duplicate/idempotent replays are deduplicated
// downstream and never delivered, so a retry of an already-received event must not
// consume cap quota nor 429 near the cap. This mirrors the source ingest path
// (api/ingest.go), which resolves the duplicate verdict before consuming quota. The
// verdict comes from the caller-supplied predicate so it cannot drift from the service
// it guards (see trialCapDuplicateVerdict). Failure policy: a duplicate-lookup error is
// treated as not-duplicate (the cap still applies), so a DB fault can at worst charge a
// duplicate against the quota; the cap itself keeps the fail-open policy documented on
// EnforceTrialEventCap.
func (h *Handler) enforceTrialEventCapForNewEvent(w http.ResponseWriter, r *http.Request, orgID, projectID, idempotencyKey string, isDuplicate trialCapDuplicateVerdict) bool {
	if h.A == nil || !h.A.Cfg.UsesOrgBilling() || h.A.TrialEvents == nil {
		return false
	}

	if !util.IsStringEmpty(idempotencyKey) {
		dup, err := isDuplicate(r.Context(), projectID, idempotencyKey)
		if err != nil {
			h.A.Logger.Warn("trial event cap: duplicate lookup failed, treating event as new", "error", err, "project_id", projectID)
		} else if dup {
			return false
		}
	}

	return EnforceTrialEventCap(w, r, h.A, orgID)
}

// EnforceTrialEventCap applies the cloud-trial daily event cap for the org that
// owns the event's project. It is the chokepoint for the HTTP event-creation
// surfaces (public API, dashboard, portal, and the source ingest endpoint),
// called from the handlers after the project is resolved rather than as route
// middleware, since portal and data-plane-UI routes do not resolve an org into
// context. Broker (pub/sub) ingestion enforces the same cap via the shared
// TrialEventLimiter in the data plane (see pubsub.Ingest.EnableTrialEventCap).
// It returns true and renders a 429 when the cap is exceeded (the caller must
// stop); it returns false for non-cloud instances, orgs with no cap, a missing
// org, an org lookup error, and Redis outages (fail open).
func EnforceTrialEventCap(w http.ResponseWriter, r *http.Request, a *types.APIOptions, orgID string) bool {
	if a == nil || !a.Cfg.UsesOrgBilling() || a.TrialEvents == nil {
		return false
	}

	org, err := trialCapOrgRepo(a).FetchOrganisationByID(r.Context(), orgID)
	if err != nil || org == nil {
		// Fail open on a lookup error or missing org: a cost cap must not hard-block
		// ingestion on a transient DB fault, matching the Redis fail-open policy in
		// TrialEventLimiter.Allow.
		if err != nil {
			a.Logger.Warn("trial event cap: org lookup failed, allowing event (fail open)", "error", err, "org_id", orgID)
		}
		return false
	}

	// Allow intentionally consumes a quota unit as part of the atomic check-and-
	// increment, before the caller enqueues the event. A post-check failure (encode,
	// queue write) therefore spends a unit without an accepted event. Chosen policy:
	// a compensating decrement on failure would race concurrent requests near the cap
	// (a decrement can readmit a request the cap already rejected), so we accept a
	// rare, self-healing loss of at most one unit out of the daily allowance instead.
	if limitErr := a.TrialEvents.Allow(r.Context(), org.UID, org.LicenseData); errors.Is(limitErr, license.ErrDailyEventLimit) {
		_ = render.Render(w, r, util.NewErrorResponse("Daily trial event limit reached. Your limit resets at 00:00 UTC, or full limits apply once your trial converts to a paid plan.", http.StatusTooManyRequests))
		return true
	}
	return false
}

// trialCapOrgCacheTTL bounds how long the cap path may serve a stale organisation
// (and therefore stale license_data). Kept short and aligned with the limiter's own
// license_data-keyed memoisation so a trial start or trial->paid conversion is picked
// up within about a minute instead of enforcing the pre-change cap for 5 minutes.
const trialCapOrgCacheTTL = time.Minute

// trialCapOrgRepo resolves the organisation repository used by the trial cap,
// preferring the injected repo (mockable in tests) and wrapping it in a short-lived
// cache on the hot ingest path when a cache is configured. The short TTL avoids a
// stale license_data window that would otherwise outlast the limiter's own cache.
func trialCapOrgRepo(a *types.APIOptions) datastore.OrganisationRepository {
	inner := a.OrgRepo
	if inner == nil {
		inner = organisations.New(a.Logger, a.DB)
	}
	if a.Cache == nil {
		return inner
	}
	return cached.NewCachedOrganisationRepository(inner, a.Cache, trialCapOrgCacheTTL, a.Logger)
}

// RequireOrganisationMembership fails closed unless the authenticated user is a
// member of the organisation resolved from the request (orgID URL/query param).
// It guards org-scoped dashboard reads against cross-org disclosure. It must not
// be mounted on the public API or instance-admin routers, which authenticate via
// API key / instance-admin role rather than org membership.
func (h *Handler) RequireOrganisationMembership() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Instance admins manage every organisation (the same trust the
			// sibling write routes grant via OrganisationPolicy), so they
			// bypass the per-org membership requirement on these reads.
			if h.isInstanceAdmin(r) {
				next.ServeHTTP(w, r)
				return
			}

			if _, err := h.retrieveMembership(r); err != nil {
				// Fail closed on every path, but distinguish a definitive
				// negative (the org or the membership does not exist -> 403)
				// from an internal/lookup failure (-> 500). Mapping a DB outage
				// to 403 would block real members and hide the fault from
				// operators; both sentinels return 403 so a non-existent org is
				// not enumerable against a real-but-foreign org.
				if errors.Is(err, datastore.ErrOrgMemberNotFound) || errors.Is(err, datastore.ErrOrgNotFound) {
					_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: must be a member of the organisation", http.StatusForbidden))
					return
				}

				h.A.Logger.Error("Failed to verify organisation membership", "error", err)
				_ = render.Render(w, r, util.NewErrorResponse("failed to verify organisation membership", http.StatusInternalServerError))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
