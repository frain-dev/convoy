package handlers

import (
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/subomi/requestmigrations"
)

type Handler struct {
	A  *types.APIOptions
	RM *requestmigrations.RequestMigration
}

func (h *Handler) IsReqWithProjectAPIKey(authUser *auth.AuthenticatedUser) bool {
	keyIsAPIKey := authUser.Credential.Type == auth.CredentialTypeAPIKey
	userIsNil := authUser.User == nil

	return keyIsAPIKey && userIsNil
}

func (h *Handler) IsReqWithPersonalAccessToken(authUser *auth.AuthenticatedUser) bool {
	keyIsAPIKey := authUser.Credential.Type == auth.CredentialTypeAPIKey
	userIsNotNil := authUser.User != nil

	return keyIsAPIKey && userIsNotNil
}

func (h *Handler) IsReqWithJWT(authUser *auth.AuthenticatedUser) bool {
	return authUser.Credential.Type == auth.CredentialTypeJWT
}

func (h *Handler) IsReqWithPortalLinkToken(authUser *auth.AuthenticatedUser) bool {
	return authUser.Credential.Type == auth.CredentialTypeToken
}

func (h *Handler) retrieveProject(r *http.Request) (*datastore.Project, error) {
	authUser := middleware.GetAuthUserFromContext(r.Context())

	var project *datastore.Project
	var err error

	projectRepo := postgres.NewProjectRepo(h.A.DB)

	switch {
	case h.IsReqWithJWT(authUser), h.IsReqWithPersonalAccessToken(authUser):
		projectID := chi.URLParam(r, "projectID")
		if util.IsStringEmpty(projectID) {
			return nil, errors.New("project id not present in request")
		}

		project, err = projectRepo.FetchProjectByID(r.Context(), projectID)
		if err != nil {
			return nil, err
		}

		if err = h.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
			return nil, err
		}
	case h.IsReqWithProjectAPIKey(authUser):
		apiKey, ok := authUser.APIKey.(*datastore.APIKey)
		if !ok {
			return nil, errors.New("invalid auth object")
		}

		projectID := apiKey.Role.Project
		project, err = projectRepo.FetchProjectByID(r.Context(), projectID)
		if err != nil {
			return nil, err
		}
	case h.IsReqWithPortalLinkToken(authUser):
		portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB)
		pLink, err := portalLinkRepo.FindPortalLinkByToken(r.Context(), authUser.Credential.Token)
		if err != nil {
			return nil, err
		}

		project, err = projectRepo.FetchProjectByID(r.Context(), pLink.ProjectID)
		if err != nil {
			return nil, err
		}

	default: // No auth, this is an impossible scenario, but fail anyways.
		return nil, errors.New("auth: auth object was not recognized")
	}

	return project, nil
}

func (h *Handler) retrieveHost() (string, error) {
	cfg, err := config.Get()
	if err != nil {
		return "", err
	}

	return cfg.Host, nil
}

func (h *Handler) retrieveOrganisation(r *http.Request) (*datastore.Organisation, error) {
	orgID := chi.URLParam(r, "orgID")

	if util.IsStringEmpty(orgID) {
		orgID = r.URL.Query().Get("orgID")
	}

	orgRepo := postgres.NewOrgRepo(h.A.DB)
	return orgRepo.FetchOrganisationByID(r.Context(), orgID)
}

func (h *Handler) retrieveMembership(r *http.Request) (*datastore.OrganisationMember, error) {
	org, err := h.retrieveOrganisation(r)
	if err != nil {
		return &datastore.OrganisationMember{}, err
	}

	user, err := h.retrieveUser(r)
	if err != nil {
		return &datastore.OrganisationMember{}, err
	}

	orgMemberRepo := postgres.NewOrgMemberRepo(h.A.DB)
	return orgMemberRepo.FetchOrganisationMemberByUserID(r.Context(), user.UID, org.UID)
}

func (h *Handler) retrieveUser(r *http.Request) (*datastore.User, error) {
	authUser := middleware.GetAuthUserFromContext(r.Context())
	user, ok := authUser.User.(*datastore.User)
	if !ok {
		return &datastore.User{}, errors.New("user not found")
	}

	return user, nil
}

func (h *Handler) retrievePortalLinkFromToken(r *http.Request) (*datastore.PortalLink, error) {
	var pLink *datastore.PortalLink
	portalLinkRepo := postgres.NewPortalLinkRepo(h.A.DB)

	authUser := middleware.GetAuthUserFromContext(r.Context())
	pLink, err := portalLinkRepo.FindPortalLinkByToken(r.Context(), authUser.Credential.Token)
	if err != nil {
		return nil, err
	}

	return pLink, nil
}

func (h *Handler) CanManageEndpoint() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			portalLink, err := h.retrievePortalLinkFromToken(r)
			if err != nil {
				_ = render.Render(w, r, util.NewServiceErrResponse(err))
				return
			}

			if !portalLink.CanManageEndpoint {
				_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
