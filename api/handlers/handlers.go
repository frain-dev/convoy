package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/subomi/requestmigrations/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/cached"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/internal/portal_links"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/subscriptions"
	"github.com/frain-dev/convoy/util"
)

type Handler struct {
	A          *types.APIOptions
	Versioning *requestmigrations.RequestMigration
}

// orgRepo returns the injected organisation repository, falling back to a freshly
// constructed one when none was wired (e.g. in tests).
func (h *Handler) orgRepo() datastore.OrganisationRepository {
	if h.A.OrgRepo != nil {
		return h.A.OrgRepo
	}
	return organisations.New(h.A.Logger, h.A.DB)
}

// projectRepo returns the injected project repository, falling back to a freshly
// constructed one when none was wired (e.g. in tests).
func (h *Handler) projectRepo() datastore.ProjectRepository {
	if h.A.ProjectRepo != nil {
		return h.A.ProjectRepo
	}
	return projects.New(h.A.Logger, h.A.DB)
}

// orgMemberRepo returns the injected organisation member repository, falling back to a
// freshly constructed one when none was wired (e.g. in tests).
func (h *Handler) orgMemberRepo() datastore.OrganisationMemberRepository {
	if h.A.OrgMemberRepo != nil {
		return h.A.OrgMemberRepo
	}
	return organisation_members.New(h.A.Logger, h.A.DB)
}

// eventRepo returns the injected event repository, falling back to a freshly
// constructed one when none was wired (e.g. in tests).
func (h *Handler) eventRepo() datastore.EventRepository {
	if h.A.EventRepo != nil {
		return h.A.EventRepo
	}
	return events.New(h.A.Logger, h.A.DB)
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

	projectRepo := cached.NewCachedProjectRepository(projects.New(h.A.Logger, h.A.DB), h.A.Cache, 5*time.Minute, h.A.Logger)

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

		if err := h.A.Authz.Authorize(r.Context(), string(policies.PermissionProjectView), project); err != nil {
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
		if len(authUser.Credential.Token) > 0 { // this is the legacy static token type
			svc := portal_links.New(h.A.Logger, h.A.DB)
			pLink, err2 := svc.GetPortalLinkByToken(r.Context(), authUser.Credential.Token)
			if err2 != nil {
				return nil, err2
			}

			project, err2 = projectRepo.FetchProjectByID(r.Context(), pLink.ProjectID)
			if err2 != nil {
				return nil, err2
			}
		} else {
			portalLink, ok := authUser.PortalLink.(*datastore.PortalLink)
			if !ok {
				return nil, errors.New("invalid auth object")
			}

			projectID := portalLink.ProjectID
			project, err = projectRepo.FetchProjectByID(r.Context(), projectID)
			if err != nil {
				return nil, err
			}

			return project, nil
		}

	default: // No auth, this is an impossible scenario, but fail anyways.
		return nil, errors.New("auth: auth object was not recognized")
	}

	return project, nil
}

// getProjectFromContext retrieves the project from context if available,
// otherwise falls back to retrieveProject(). This avoids redundant database
// queries when middleware has already loaded the project.
func (h *Handler) getProjectFromContext(r *http.Request) (*datastore.Project, error) {
	// First check if project is already in context (set by RequireEnabledProject middleware)
	if cachedProject := r.Context().Value(convoy.ProjectCtx); cachedProject != nil {
		return cachedProject.(*datastore.Project), nil
	}

	// Fall back to retrieveProject for cases without middleware
	return h.retrieveProject(r)
}

func (h *Handler) retrieveHost() (string, error) {
	cfg, err := config.Get()
	if err != nil {
		return "", err
	}

	return cfg.Host, nil
}

func (h *Handler) retrieveOrganisation(r *http.Request) (*datastore.Organisation, error) {
	var (
		org *datastore.Organisation
		err error
	)

	orgID := chi.URLParam(r, "orgID")
	if util.IsStringEmpty(orgID) {
		orgID = r.URL.Query().Get("orgID")
	}
	if !util.IsStringEmpty(orgID) {
		orgRepo := organisations.New(h.A.Logger, h.A.DB)
		org, err = orgRepo.FetchOrganisationByID(r.Context(), orgID)
		if err == nil && org != nil {
			return org, nil
		}
	}

	if cachedOrg := r.Context().Value(convoy.OrganisationCtx); cachedOrg != nil {
		org = cachedOrg.(*datastore.Organisation)
	} else if cachedProject := r.Context().Value(convoy.ProjectCtx); cachedProject != nil {
		project := cachedProject.(*datastore.Project)
		orgRepo := organisations.New(h.A.Logger, h.A.DB)
		org, err = orgRepo.FetchOrganisationByID(r.Context(), project.OrganisationID)
	} else if projectID := chi.URLParam(r, "projectID"); projectID != "" {
		projectRepo := projects.New(h.A.Logger, h.A.DB)
		var project *datastore.Project
		project, err = projectRepo.FetchProjectByID(r.Context(), projectID)
		if err == nil {
			orgRepo := organisations.New(h.A.Logger, h.A.DB)
			org, err = orgRepo.FetchOrganisationByID(r.Context(), project.OrganisationID)
		}
	}

	if err != nil || org == nil {
		return nil, err
	}

	return org, nil
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

	orgMemberRepo := organisation_members.New(h.A.Logger, h.A.DB)
	return orgMemberRepo.FetchOrganisationMemberByUserID(r.Context(), user.UID, org.UID)
}

func (h *Handler) retrieveUser(r *http.Request) (*datastore.User, error) {
	// Comma-ok on the context value directly (middleware.GetAuthUserFromContext does an
	// unchecked assertion that panics on unauthenticated routes). Callers that need a user
	// already handle this error; the license display path relies on it to fail open (no
	// user => orgCount stays -1, never gated) on portal/no-user routes.
	authUser, ok := r.Context().Value(convoy.AuthUserCtx).(*auth.AuthenticatedUser)
	if !ok || authUser == nil {
		return &datastore.User{}, errors.New("user not found")
	}
	user, ok := authUser.User.(*datastore.User)
	if !ok {
		return &datastore.User{}, errors.New("user not found")
	}

	return user, nil
}

func (h *Handler) retrievePortalLinkFromToken(r *http.Request) (*datastore.PortalLink, error) {
	var pLink *datastore.PortalLink
	var err error

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if len(authUser.Credential.Token) > 0 { // this is the legacy static token type
		svc := portal_links.New(h.A.Logger, h.A.DB)
		pLink, err = svc.GetPortalLinkByToken(r.Context(), authUser.Credential.Token)
		if err != nil {
			return nil, err
		}
	} else {
		portalLink, ok := authUser.PortalLink.(*datastore.PortalLink)
		if !ok {
			return nil, errors.New("invalid auth object")
		}

		return portalLink, nil
	}

	return pLink, nil
}

// portalLinkOwnedEndpointIDs resolves the endpoint ids owned by the portal link behind
// the current request. The isPortal return reports whether the request actually uses a
// portal-link token; for non-portal requests (JWT / API key / dashboard) it is false and
// the caller must skip owner scoping. On a resolution error it writes the error response
// and returns ok=false; callers must stop processing in that case.
func (h *Handler) portalLinkOwnedEndpointIDs(w http.ResponseWriter, r *http.Request, authUser *auth.AuthenticatedUser) (ownedIDs []string, isPortal, ok bool) {
	if !h.IsReqWithPortalLinkToken(authUser) {
		return nil, false, true
	}

	portalLink, err := h.retrievePortalLinkFromToken(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return nil, true, false
	}

	ownedIDs, err = h.getEndpoints(r, portalLink)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return nil, true, false
	}

	return ownedIDs, true, true
}

// ensurePortalLinkOwnsEndpoints enforces portal-link owner scoping for by-id and
// sub-resource handlers reachable on /portal-api. For a portal-link request EVERY
// supplied endpoint id must belong to the portal link's owner; for non-portal requests
// it is a no-op so admin / API access is unchanged. It writes the error response and
// returns false when the request must be stopped.
//
// Failure policy: fail closed. An empty id list, an empty id, or any id not owned by the
// caller is rejected with 401. Use this for resources tied to exactly one endpoint
// (endpoints, subscriptions, event deliveries, filters) and for batch operations where
// every targeted resource must belong to the caller.
func (h *Handler) ensurePortalLinkOwnsEndpoints(w http.ResponseWriter, r *http.Request, authUser *auth.AuthenticatedUser, endpointIDs ...string) bool {
	ownedIDs, isPortal, ok := h.portalLinkOwnedEndpointIDs(w, r, authUser)
	if !ok {
		return false
	}
	if !isPortal {
		return true
	}

	if len(endpointIDs) == 0 {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
		return false
	}

	for _, endpointID := range endpointIDs {
		if util.IsStringEmpty(endpointID) || !util.StringSliceContains(ownedIDs, endpointID) {
			_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
			return false
		}
	}

	return true
}

// ensurePortalLinkOwnsAnyEndpoint enforces portal-link owner scoping for resources that
// can be associated with several endpoints (events). For a portal-link request AT LEAST
// ONE supplied endpoint id must be owned by the portal link, mirroring the list handlers
// which surface an event when any of its endpoints is owned by the caller. No-op for
// non-portal requests.
//
// Failure policy: fail closed. If none of the ids are owned (or the list is empty) the
// request is rejected with 401.
func (h *Handler) ensurePortalLinkOwnsAnyEndpoint(w http.ResponseWriter, r *http.Request, authUser *auth.AuthenticatedUser, endpointIDs ...string) bool {
	ownedIDs, isPortal, ok := h.portalLinkOwnedEndpointIDs(w, r, authUser)
	if !ok {
		return false
	}
	if !isPortal {
		return true
	}

	for _, endpointID := range endpointIDs {
		if !util.IsStringEmpty(endpointID) && util.StringSliceContains(ownedIDs, endpointID) {
			return true
		}
	}

	_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
	return false
}

// requirePortalLinkOwnsSubscription is route middleware that, for portal-link requests,
// verifies the {subscriptionID} in the path belongs to an endpoint owned by the portal
// link before the wrapped (filter) handlers run. It centralizes owner scoping for the
// filter sub-resource so every current and future filter route shares one check. No-op
// for non-portal (JWT / API key) requests. Fail closed: a missing or foreign
// subscription is rejected before the handler runs.
func (h *Handler) RequirePortalLinkOwnsSubscription() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := middleware.GetAuthUserFromContext(r.Context())
			if h.IsReqWithPortalLinkToken(authUser) {
				project, err := h.retrieveProject(r)
				if err != nil {
					_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
					return
				}

				sub, err := subscriptions.New(h.A.Logger, h.A.DB).FindSubscriptionByID(r.Context(), project.UID, chi.URLParam(r, "subscriptionID"))
				if err != nil {
					if errors.Is(err, datastore.ErrSubscriptionNotFound) {
						_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
						return
					}
					_ = render.Render(w, r, util.NewServiceErrResponse(err))
					return
				}

				if !h.ensurePortalLinkOwnsEndpoints(w, r, authUser, sub.EndpointID) {
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
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

func (h *Handler) isOrganisationDisabled(org *datastore.Organisation) bool {
	return org.DisabledAt.Valid && org.DisabledAt.Time.After(time.Unix(0, 0))
}
