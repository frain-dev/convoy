package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/util"
)

var ErrProjectDisabled = errors.New("this project has been disabled for write operations until you re-subscribe your convoy instance")

func (h *Handler) RequireEnabledProject() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, err := h.retrieveProject(r)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse("failed to retrieve project", http.StatusBadRequest))
				return
			}

			if !h.A.Licenser.ProjectEnabled(p.UID) {
				_ = render.Render(w, r, util.NewErrorResponse(ErrProjectDisabled.Error(), http.StatusBadRequest))
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
					orgRepo := organisations.New(h.A.Logger, h.A.DB)
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

			if h.A.Cfg.Billing.Enabled && h.isOrganisationDisabled(org) {
				_ = render.Render(w, r, util.NewErrorResponse("This action is disabled for this organization. Please contact support or subscribe to a plan.", http.StatusForbidden))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
