package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy"
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
