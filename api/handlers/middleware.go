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

			projectID := chi.URLParam(r, "projectID")
			if projectID != "" {
				var project *datastore.Project

				if cachedProject := r.Context().Value(convoy.ProjectCtx); cachedProject != nil {
					project = cachedProject.(*datastore.Project)
				} else {
					project, err = h.retrieveProject(r)
					if err != nil {
						_ = render.Render(w, r, util.NewErrorResponse("failed to retrieve project", http.StatusBadRequest))
						return
					}
				}

				if cachedOrg := r.Context().Value(convoy.OrganisationCtx); cachedOrg != nil {
					org = cachedOrg.(*datastore.Organisation)
				} else {
					orgRepo := organisations.New(h.A.Logger, h.A.DB)
					org, err = orgRepo.FetchOrganisationByID(r.Context(), project.OrganisationID)
					if err != nil {
						h.A.Logger.WithError(err).Error("Failed to fetch organisation for disabled check")
						_ = render.Render(w, r, util.NewErrorResponse("Failed to verify organization status", http.StatusInternalServerError))
						return
					}
					ctx := context.WithValue(r.Context(), convoy.OrganisationCtx, org)
					r = r.WithContext(ctx)
				}
			} else {
				if cachedOrg := r.Context().Value(convoy.OrganisationCtx); cachedOrg != nil {
					org = cachedOrg.(*datastore.Organisation)
				} else {
					org, err = h.retrieveOrganisation(r)
					if err != nil {
						_ = render.Render(w, r, util.NewServiceErrResponse(err))
						return
					}
					ctx := context.WithValue(r.Context(), convoy.OrganisationCtx, org)
					r = r.WithContext(ctx)
				}
			}

			if h.isOrganisationDisabled(org) {
				_ = render.Render(w, r, util.NewErrorResponse("This action is disabled for this organization. Please contact support or subscribe to a plan.", http.StatusForbidden))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
