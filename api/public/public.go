package public

import (
	"net/http"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

type PublicHandler struct {
	M      *middleware.Middleware
	Router http.Handler
	A      types.App
}

func NewPublicHandler(a types.App) *PublicHandler {
	m := middleware.NewMiddleware(&middleware.CreateMiddleware{
		Cache:             a.Cache,
		Logger:            a.Logger,
		Limiter:           a.Limiter,
		Tracer:            a.Tracer,
		EventRepo:         postgres.NewEventRepo(a.DB),
		EventDeliveryRepo: postgres.NewEventDeliveryRepo(a.DB),
		EndpointRepo:      postgres.NewEndpointRepo(a.DB),
		ProjectRepo:       postgres.NewProjectRepo(a.DB),
		ApiKeyRepo:        postgres.NewAPIKeyRepo(a.DB),
		SubRepo:           postgres.NewSubscriptionRepo(a.DB),
		SourceRepo:        postgres.NewSourceRepo(a.DB),
		OrgRepo:           postgres.NewOrgRepo(a.DB),
		OrgMemberRepo:     postgres.NewOrgMemberRepo(a.DB),
		OrgInviteRepo:     postgres.NewOrgInviteRepo(a.DB),
		UserRepo:          postgres.NewUserRepo(a.DB),
		ConfigRepo:        postgres.NewConfigRepo(a.DB),
		DeviceRepo:        postgres.NewDeviceRepo(a.DB),
		PortalLinkRepo:    postgres.NewPortalLinkRepo(a.DB),
	})

	return &PublicHandler{
		M: m,
		A: types.App{
			DB:       a.DB,
			Queue:    a.Queue,
			Cache:    a.Cache,
			Searcher: a.Searcher,
			Logger:   a.Logger,
			Tracer:   a.Tracer,
			Limiter:  a.Limiter,
		},
	}
}

func (a *PublicHandler) BuildRoutes() http.Handler {
	router := chi.NewRouter()

	router.Route("/v1", func(r chi.Router) {
		r.Use(chiMiddleware.AllowContentType("application/json"))
		r.Use(a.M.JsonResponse)
		r.Use(a.M.RequireAuth())

		r.With(a.M.Pagination, a.M.RequireAuthUserMetadata()).Get("/organisations", a.GetOrganisationsPaged)

		r.Route("/projects", func(projectRouter chi.Router) {
			projectRouter.Use(a.M.RejectAppPortalKey())

			// These routes require a Personal API Key or JWT Token to work
			projectRouter.With(
				a.M.RequireAuthUserMetadata(),
				a.M.RequireOrganisation(),
				a.M.RequireOrganisationMembership(),
				a.M.RequireOrganisationMemberRole(auth.RoleSuperUser),
			).Post("/", a.CreateProject)

			projectRouter.With(
				a.M.RequireAuthUserMetadata(),
				a.M.RequireOrganisation(),
				a.M.RequireOrganisationMembership(),
			).Get("/", a.GetProjects)

			projectRouter.Route("/{projectID}", func(projectSubRouter chi.Router) {
				projectSubRouter.Use(a.M.RequireProject())
				projectSubRouter.Use(a.M.RequireProjectAccess())

				projectSubRouter.With().Get("/", a.GetProject)
				projectSubRouter.Put("/", a.UpdateProject)
				projectSubRouter.Delete("/", a.DeleteProject)

				projectSubRouter.Route("/endpoints", func(endpointSubRouter chi.Router) {
					endpointSubRouter.Use(a.M.RateLimitByProjectID())

					endpointSubRouter.Post("/", a.CreateEndpoint)
					endpointSubRouter.With(a.M.Pagination).Get("/", a.GetEndpoints)

					endpointSubRouter.Route("/{endpointID}", func(e chi.Router) {
						e.Use(a.M.RequireEndpoint())
						e.Use(a.M.RequireEndpointBelongsToProject())

						e.Get("/", a.GetEndpoint)
						e.Put("/", a.UpdateEndpoint)
						e.Delete("/", a.DeleteEndpoint)
						e.Put("/expire_secret", a.ExpireSecret)
						e.Put("/toggle_status", a.ToggleEndpointStatus)
					})
				})

				projectSubRouter.Route("/applications", func(appRouter chi.Router) {
					appRouter.Use(a.M.RateLimitByProjectID())

					appRouter.Post("/", a.CreateApp)
					appRouter.With(a.M.Pagination).Get("/", a.GetApps)

					appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
						appSubRouter.Use(a.M.RequireApp())
						appSubRouter.Use(a.M.RequireAppBelongsToProject())

						appSubRouter.Get("/", a.GetApp)
						appSubRouter.Put("/", a.UpdateApp)
						appSubRouter.Delete("/", a.DeleteApp)

						appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
							endpointAppSubRouter.Post("/", a.CreateAppEndpoint)
							endpointAppSubRouter.Get("/", a.GetAppEndpoints)

							endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
								e.Use(a.M.RequireAppEndpoint())

								e.Get("/", a.GetAppEndpoint)
								e.Put("/", a.UpdateAppEndpoint)
								e.Delete("/", a.DeleteAppEndpoint)
								e.Put("/expire_secret", a.ExpireSecret)
							})
						})
					})
				})

				projectSubRouter.Route("/events", func(eventRouter chi.Router) {
					eventRouter.Use(a.M.RateLimitByProjectID())

					// TODO(all): should the InstrumentPath change?
					eventRouter.With(a.M.InstrumentPath("/events")).Post("/", a.CreateEndpointEvent)
					eventRouter.Post("/fanout", a.CreateEndpointFanoutEvent)
					eventRouter.With(a.M.Pagination).Get("/", a.GetEventsPaged)
					eventRouter.Post("/batchreplay", a.BatchReplayEvents)
					eventRouter.Get("/countbatchreplayevents", a.CountAffectedEvents)

					eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
						eventSubRouter.Use(a.M.RequireEvent())
						eventSubRouter.Get("/", a.GetEndpointEvent)
						eventSubRouter.Put("/replay", a.ReplayEndpointEvent)
					})
				})

				projectSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
					eventDeliveryRouter.With(a.M.Pagination).Get("/", a.GetEventDeliveriesPaged)
					eventDeliveryRouter.Post("/forceresend", a.ForceResendEventDeliveries)
					eventDeliveryRouter.Post("/batchretry", a.BatchRetryEventDelivery)
					eventDeliveryRouter.Get("/countbatchretryevents", a.CountAffectedEventDeliveries)

					eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
						eventDeliverySubRouter.Use(a.M.RequireEventDelivery())

						eventDeliverySubRouter.Get("/", a.GetEventDelivery)
						eventDeliverySubRouter.Put("/resend", a.ResendEventDelivery)

						eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
							deliveryRouter.Use(fetchDeliveryAttempts())

							deliveryRouter.Get("/", a.GetDeliveryAttempts)
							deliveryRouter.With(a.M.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", a.GetDeliveryAttempt)
						})
					})
				})

				projectSubRouter.Route("/security", func(securityRouter chi.Router) {
					securityRouter.Route("/endpoints/{endpointID}/keys", func(securitySubRouter chi.Router) {
						securitySubRouter.Use(a.M.RequireEndpoint())
						securitySubRouter.Use(a.M.RequireBaseUrl())
						securitySubRouter.With(fflag.CanAccessFeature(fflag.Features[fflag.CanCreateCLIAPIKey])).Post("/", a.CreateEndpointAPIKey)
					})
				})

				projectSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
					subscriptionRouter.Use(a.M.RateLimitByProjectID())

					subscriptionRouter.Post("/", a.CreateSubscription)
					subscriptionRouter.Post("/test_filter", a.TestSubscriptionFilter)
					subscriptionRouter.With(a.M.Pagination, a.M.RequireBaseUrl()).Get("/", a.GetSubscriptions)
					subscriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
					subscriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
					subscriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
					subscriptionRouter.Put("/{subscriptionID}/toggle_status", a.ToggleSubscriptionStatus)
				})

				projectSubRouter.Route("/sources", func(sourceRouter chi.Router) {
					sourceRouter.Use(a.M.RequireBaseUrl())

					sourceRouter.Post("/", a.CreateSource)
					sourceRouter.Get("/{sourceID}", a.GetSourceByID)
					sourceRouter.With(a.M.Pagination).Get("/", a.LoadSourcesPaged)
					sourceRouter.Put("/{sourceID}", a.UpdateSource)
					sourceRouter.Delete("/{sourceID}", a.DeleteSource)
				})

				projectSubRouter.Route("/portal-links", func(portalLinkRouter chi.Router) {
					portalLinkRouter.Use(a.M.RequireBaseUrl())

					portalLinkRouter.Post("/", a.CreatePortalLink)
					portalLinkRouter.Get("/{portalLinkID}", a.GetPortalLinkByID)
					portalLinkRouter.With(a.M.Pagination).Get("/", a.LoadPortalLinksPaged)
					portalLinkRouter.Put("/{portalLinkID}", a.UpdatePortalLink)
					portalLinkRouter.Put("/{portalLinkID}/revoke", a.RevokePortalLink)
				})
			})
		})

		r.HandleFunc("/*", a.RedirectToProjects)
	})

	return router
}
