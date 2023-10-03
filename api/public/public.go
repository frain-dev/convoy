package public

import (
	"errors"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"net/http"
	"time"
)

type PublicHandler struct {
	Router http.Handler
	A      *types.APIOptions
}

func NewPublicHandler(a *types.APIOptions) *PublicHandler {
	return &PublicHandler{A: a}
}

func (a *PublicHandler) BuildRoutes() http.Handler {
	router := chi.NewRouter()

	router.Route("/v1", func(r chi.Router) {
		r.Use(chiMiddleware.AllowContentType("application/json"))
		r.Use(middleware.JsonResponse)
		r.Use(middleware.RequireAuth())

		r.With(middleware.Pagination, RequirePersonalAPIKeys(a)).Get("/organisations", a.GetOrganisationsPaged)

		r.Route("/projects", func(projectRouter chi.Router) {
			// These routes require a Personal API Key.
			projectRouter.With(RequirePersonalAPIKeys(a)).Get("/", a.GetProjects)
			projectRouter.With(RequirePersonalAPIKeys(a)).Post("/", a.CreateProject)

			projectRouter.Route("/{projectID}", func(projectSubRouter chi.Router) {
				projectSubRouter.Use(RequireProjectAccess(a))
				projectSubRouter.Get("/", a.GetProject)
				projectSubRouter.Put("/", a.UpdateProject)
				projectSubRouter.Delete("/", a.DeleteProject)

				projectSubRouter.Route("/endpoints", func(endpointSubRouter chi.Router) {
					endpointSubRouter.Post("/", a.CreateEndpoint)
					endpointSubRouter.With(middleware.Pagination).Get("/", a.GetEndpoints)

					endpointSubRouter.Route("/{endpointID}", func(e chi.Router) {
						e.Get("/", a.GetEndpoint)
						e.Put("/", a.UpdateEndpoint)
						e.Delete("/", a.DeleteEndpoint)
						e.Put("/expire_secret", a.ExpireSecret)
						e.Put("/toggle_status", a.ToggleEndpointStatus)
						e.Put("/pause", a.PauseEndpoint)
					})
				})

				projectSubRouter.Route("/events", func(eventRouter chi.Router) {
					// TODO(all): should the InstrumentPath change?
					eventRouter.With(middleware.InstrumentPath("/events")).Post("/", a.CreateEndpointEvent)
					eventRouter.Post("/fanout", a.CreateEndpointFanoutEvent)
					eventRouter.Post("/dynamic", a.CreateDynamicEvent)
					eventRouter.With(middleware.Pagination).Get("/", a.GetEventsPaged)
					eventRouter.Post("/batchreplay", a.BatchReplayEvents)

					eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
						eventSubRouter.Get("/", a.GetEndpointEvent)
						eventSubRouter.Put("/replay", a.ReplayEndpointEvent)
					})
				})

				projectSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
					eventDeliveryRouter.With(middleware.Pagination).Get("/", a.GetEventDeliveriesPaged)
					eventDeliveryRouter.Post("/forceresend", a.ForceResendEventDeliveries)
					eventDeliveryRouter.Post("/batchretry", a.BatchRetryEventDelivery)

					eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
						eventDeliverySubRouter.Get("/", a.GetEventDelivery)
						eventDeliverySubRouter.Put("/resend", a.ResendEventDelivery)

						eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
							deliveryRouter.Get("/", a.GetDeliveryAttempts)
							deliveryRouter.Get("/{deliveryAttemptID}", a.GetDeliveryAttempt)
						})
					})
				})

				projectSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
					subscriptionRouter.Post("/", a.CreateSubscription)
					subscriptionRouter.Post("/test_filter", a.TestSubscriptionFilter)
					subscriptionRouter.With(middleware.Pagination).Get("/", a.GetSubscriptions)
					subscriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
					subscriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
					subscriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
					subscriptionRouter.Put("/{subscriptionID}/toggle_status", a.ToggleSubscriptionStatus)
				})

				projectSubRouter.Route("/sources", func(sourceRouter chi.Router) {
					sourceRouter.Post("/", a.CreateSource)
					sourceRouter.Get("/{sourceID}", a.GetSourceByID)
					sourceRouter.With(middleware.Pagination).Get("/", a.LoadSourcesPaged)
					sourceRouter.Put("/{sourceID}", a.UpdateSource)
					sourceRouter.Delete("/{sourceID}", a.DeleteSource)
				})

				projectSubRouter.Route("/portal-links", func(portalLinkRouter chi.Router) {
					portalLinkRouter.Post("/", a.CreatePortalLink)
					portalLinkRouter.Get("/{portalLinkID}", a.GetPortalLinkByID)
					portalLinkRouter.With(middleware.Pagination).Get("/", a.LoadPortalLinksPaged)
					portalLinkRouter.Put("/{portalLinkID}", a.UpdatePortalLink)
					portalLinkRouter.Put("/{portalLinkID}/revoke", a.RevokePortalLink)
				})

				projectSubRouter.Route("/meta-events", func(metaEventRouter chi.Router) {
					metaEventRouter.With(middleware.Pagination).Get("/", a.GetMetaEventsPaged)

					metaEventRouter.Route("/{metaEventID}", func(metaEventSubRouter chi.Router) {
						metaEventSubRouter.Get("/", a.GetMetaEvent)
						metaEventSubRouter.Put("/resend", a.ResendMetaEvent)
					})
				})
			})
		})

		r.HandleFunc("/*", a.RedirectToProjects)
	})

	return router
}

func (a *PublicHandler) retrieveOrganisation(r *http.Request) (*datastore.Organisation, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return nil, err
	}

	orgRepo := postgres.NewOrgRepo(a.A.DB)
	return orgRepo.FetchOrganisationByID(r.Context(), project.OrganisationID)
}

func (a *PublicHandler) retrieveProject(r *http.Request) (*datastore.Project, error) {
	projectID := chi.URLParam(r, "projectID")

	if util.IsStringEmpty(projectID) {
		return nil, errors.New("project id not present in request")
	}

	var project *datastore.Project
	projectCacheKey := convoy.ProjectsCacheKey.Get(projectID).String()
	err := a.A.Cache.Get(r.Context(), projectCacheKey, &project)
	if err != nil {
		return nil, err
	}

	if project != nil {
		return project, nil
	}

	// fetch project from context or cache
	projectRepo := postgres.NewProjectRepo(a.A.DB)
	project, err = projectRepo.FetchProjectByID(r.Context(), projectID)
	if err != nil {
		return nil, err
	}

	err = a.A.Cache.Set(r.Context(), projectCacheKey, &project, time.Minute*10)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (a *PublicHandler) retrieveUser(r *http.Request) (*datastore.User, error) {
	authCtx := r.Context().Value(middleware.AuthUserCtx).(*auth.AuthenticatedUser)

	user, ok := authCtx.User.(*datastore.User)
	if !ok {
		return &datastore.User{}, errors.New("User not found")
	}

	return user, nil
}

func (a *PublicHandler) retrieveHost() (string, error) {
	cfg, err := config.Get()
	if err != nil {
		return "", err
	}

	return cfg.Host, nil
}

func RequireProjectAccess(a *PublicHandler) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			project, err := a.retrieveProject(r)
			if err != nil {
				_ = render.Render(w, r, util.NewServiceErrResponse(err))
				return
			}

			err = a.A.Authz.Authorize(r.Context(), "project.manage", project)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequirePersonalAPIKeys(a *PublicHandler) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := middleware.GetAuthUserFromContext(r.Context())
			_, ok := authUser.User.(*datastore.User)
			if !ok {
				_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
