package server

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/fflag/flipt"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/tracer"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type ApplicationHandler struct {
	M      *middleware.Middleware
	Router http.Handler
	A      App
}

type App struct {
	Store    datastore.Store
	Queue    queue.Queuer
	Logger   log.StdLogger
	Tracer   tracer.Tracer
	Cache    cache.Cache
	Limiter  limiter.RateLimiter
	Searcher searcher.Searcher
}

//go:embed ui/build
var reactFS embed.FS

func reactRootHandler(rw http.ResponseWriter, req *http.Request) {
	p := req.URL.Path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
		req.URL.Path = p
	}
	p = path.Clean(p)
	f := fs.FS(reactFS)
	static, err := fs.Sub(f, "ui/build")
	if err != nil {
		return
	}
	if _, err := static.Open(strings.TrimLeft(p, "/")); err != nil { // If file not found server index/html from root
		req.URL.Path = "/"
	}
	http.FileServer(http.FS(static)).ServeHTTP(rw, req)
}

func NewApplicationHandler(a App) *ApplicationHandler {
	m := middleware.NewMiddleware(&middleware.CreateMiddleware{
		Cache:             a.Cache,
		Logger:            a.Logger,
		Limiter:           a.Limiter,
		Tracer:            a.Tracer,
		EventRepo:         cm.NewEventRepository(a.Store),
		EventDeliveryRepo: cm.NewEventDeliveryRepository(a.Store),
		EndpointRepo:      cm.NewEndpointRepo(a.Store),
		GroupRepo:         cm.NewGroupRepo(a.Store),
		ApiKeyRepo:        cm.NewApiKeyRepo(a.Store),
		SubRepo:           cm.NewSubscriptionRepo(a.Store),
		SourceRepo:        cm.NewSourceRepo(a.Store),
		OrgRepo:           cm.NewOrgRepo(a.Store),
		OrgMemberRepo:     cm.NewOrgMemberRepo(a.Store),
		OrgInviteRepo:     cm.NewOrgInviteRepo(a.Store),
		UserRepo:          cm.NewUserRepo(a.Store),
		ConfigRepo:        cm.NewConfigRepo(a.Store),
		DeviceRepo:        cm.NewDeviceRepository(a.Store),
		PortalLinkRepo:    cm.NewPortalLinkRepo(a.Store),
	})

	return &ApplicationHandler{
		M: m,
		A: App{
			Store:    a.Store,
			Queue:    a.Queue,
			Cache:    a.Cache,
			Searcher: a.Searcher,
			Logger:   a.Logger,
			Tracer:   a.Tracer,
			Limiter:  a.Limiter,
		},
	}
}

func (a *ApplicationHandler) BuildRoutes() http.Handler {
	router := chi.NewRouter()

	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.Recoverer)
	router.Use(a.M.WriteRequestIDHeader)
	router.Use(a.M.InstrumentRequests())
	router.Use(a.M.LogHttpRequest())

	// Ingestion API
	router.Route("/ingest", func(ingestRouter chi.Router) {
		ingestRouter.Get("/{maskID}", a.HandleCrcCheck)
		ingestRouter.Post("/{maskID}", a.IngestEvent)
	})

	// Public API.
	router.Route("/api", func(v1Router chi.Router) {
		v1Router.Route("/v1", func(r chi.Router) {
			r.Use(chiMiddleware.AllowContentType("application/json"))
			r.Use(a.M.JsonResponse)
			r.Use(a.M.RequireAuth())

			r.With(a.M.Pagination, a.M.RequireAuthUserMetadata()).Get("/organisations", a.GetOrganisationsPaged)

			r.Route("/projects", func(projectRouter chi.Router) {
				projectRouter.Use(a.M.RejectAppPortalKey())
				// projectWithAuthUserRouter routes require a Personal API Key or JWT Token to work

				projectRouter.With(
					a.M.RequireAuthUserMetadata(),
					a.M.RequireOrganisation(),
					a.M.RequireOrganisationMembership(),
					a.M.RequireOrganisationMemberRole(auth.RoleSuperUser),
				).Post("/", a.CreateGroup)

				projectRouter.With(
					a.M.RequireAuthUserMetadata(),
					a.M.RequireOrganisation(),
					a.M.RequireOrganisationMembership(),
				).Get("/", a.GetGroups)

				projectRouter.Route("/{projectID}", func(projectSubRouter chi.Router) {
					projectSubRouter.Use(a.M.RequireGroup())
					projectSubRouter.Use(a.M.RequireGroupAccess())

					projectSubRouter.With().Get("/", a.GetGroup)
					projectSubRouter.Put("/", a.UpdateGroup)
					projectSubRouter.Delete("/", a.DeleteGroup)

					projectSubRouter.Route("/endpoints", func(endpointSubRouter chi.Router) {
						endpointSubRouter.Use(a.M.RateLimitByGroupID())

						endpointSubRouter.Post("/", a.CreateEndpoint)
						endpointSubRouter.With(a.M.Pagination).Get("/", a.GetEndpoints)

						endpointSubRouter.Route("/{endpointID}", func(e chi.Router) {
							e.Use(a.M.RequireEndpoint())
							e.Use(a.M.RequireEndpointBelongsToGroup())

							e.Get("/", a.GetEndpoint)
							e.Put("/", a.UpdateEndpoint)
							e.Delete("/", a.DeleteEndpoint)
							e.Put("/expire_secret", a.ExpireSecret)
						})

					})

					projectSubRouter.Route("/applications", func(appRouter chi.Router) {
						appRouter.Use(a.M.RateLimitByGroupID())

						appRouter.Post("/", a.CreateApp)
						appRouter.With(a.M.Pagination).Get("/", a.GetApps)

						appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
							appSubRouter.Use(a.M.RequireApp())
							appSubRouter.Use(a.M.RequireAppBelongsToGroup())

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
						eventRouter.Use(a.M.RateLimitByGroupID())

						// TODO(all): should the InstrumentPath change?
						eventRouter.With(a.M.InstrumentPath("/events")).Post("/", a.CreateEndpointEvent)
						eventRouter.Post("/fanout", a.CreateEndpointFanoutEvent)
						eventRouter.With(a.M.Pagination).Get("/", a.GetEventsPaged)

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
							securitySubRouter.Use(a.M.RequireEndpointBelongsToGroup())
							securitySubRouter.Use(a.M.RequireBaseUrl())
							securitySubRouter.With(fflag.CanAccessFeature(fflag.Features[fflag.CanCreateCLIAPIKey])).Post("/", a.CreateEndpointAPIKey)
						})
					})

					projectSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
						subscriptionRouter.Use(a.M.RateLimitByGroupID())

						subscriptionRouter.Post("/", a.CreateSubscription)
						subscriptionRouter.Post("/test_filter", a.TestSubscriptionFilter)
						subscriptionRouter.With(a.M.Pagination).Get("/", a.GetSubscriptions)
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
	})

	// UI API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(a.M.JsonResponse)
		uiRouter.Use(a.M.SetupCORS)
		uiRouter.Use(chiMiddleware.Maybe(a.M.RequireAuth(), middleware.ShouldAuthRoute))
		uiRouter.Use(a.M.RequireBaseUrl())

		uiRouter.Post("/organisations/process_invite", a.ProcessOrganisationMemberInvite)
		uiRouter.Get("/users/token", a.FindUserByInviteToken)

		uiRouter.Route("/users", func(userRouter chi.Router) {
			userRouter.Use(a.M.RequireAuthUserMetadata())
			userRouter.Route("/{userID}", func(userSubRouter chi.Router) {
				userSubRouter.Use(a.M.RequireAuthorizedUser())
				userSubRouter.Get("/profile", a.GetUser)
				userSubRouter.Put("/profile", a.UpdateUser)
				userSubRouter.Put("/password", a.UpdatePassword)

				userSubRouter.Route("/security/personal_api_keys", func(securityRouter chi.Router) {
					securityRouter.Post("/", a.CreatePersonalAPIKey)
					securityRouter.Put("/{keyID}/revoke", a.RevokePersonalAPIKey)
					securityRouter.With(a.M.Pagination).Get("/", a.GetAPIKeys)
				})
			})
		})

		uiRouter.Post("/users/forgot-password", a.ForgotPassword)
		uiRouter.Post("/users/reset-password", a.ResetPassword)

		uiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.Post("/login", a.LoginUser)
			authRouter.Post("/register", a.RegisterUser)
			authRouter.Post("/token/refresh", a.RefreshToken)
			authRouter.Post("/logout", a.LogoutUser)
		})

		uiRouter.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Use(a.M.RequireAuthUserMetadata())
			orgRouter.Use(a.M.RequireBaseUrl())

			orgRouter.Post("/", a.CreateOrganisation)
			orgRouter.With(a.M.Pagination).Get("/", a.GetOrganisationsPaged)

			orgRouter.Route("/{orgID}", func(orgSubRouter chi.Router) {
				orgSubRouter.Use(a.M.RequireOrganisation())
				orgSubRouter.Use(a.M.RequireOrganisationMembership())

				orgSubRouter.Get("/", a.GetOrganisation)
				orgSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", a.UpdateOrganisation)
				orgSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", a.DeleteOrganisation)

				orgSubRouter.Route("/invites", func(orgInvitesRouter chi.Router) {
					orgInvitesRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", a.InviteUserToOrganisation)
					orgInvitesRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/resend", a.ResendOrganizationInvite)
					orgInvitesRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/cancel", a.CancelOrganizationInvite)
					orgInvitesRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).With(a.M.Pagination).Get("/pending", a.GetPendingOrganisationInvites)
				})

				orgSubRouter.Route("/members", func(orgMemberRouter chi.Router) {
					orgMemberRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

					orgMemberRouter.With(a.M.Pagination).Get("/", a.GetOrganisationMembers)

					orgMemberRouter.Route("/{memberID}", func(orgMemberSubRouter chi.Router) {
						orgMemberSubRouter.Get("/", a.GetOrganisationMember)
						orgMemberSubRouter.Put("/", a.UpdateOrganisationMember)
						orgMemberSubRouter.Delete("/", a.DeleteOrganisationMember)
					})
				})

				orgSubRouter.Route("/projects", func(groupRouter chi.Router) {
					groupRouter.Route("/", func(orgSubRouter chi.Router) {
						groupRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", a.CreateGroup)
						groupRouter.Get("/", a.GetGroups)
					})

					groupRouter.Route("/{projectID}", func(groupSubRouter chi.Router) {
						groupSubRouter.Use(a.M.RequireGroup())
						groupSubRouter.Use(a.M.RateLimitByGroupID())
						groupSubRouter.Use(a.M.RequireOrganisationGroupMember())

						groupSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Get("/", a.GetGroup)
						groupSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", a.UpdateGroup)
						groupSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", a.DeleteGroup)

						groupSubRouter.Route("/endpoints", func(endpointSubRouter chi.Router) {
							endpointSubRouter.Post("/", a.CreateEndpoint)
							endpointSubRouter.With(a.M.Pagination).Get("/", a.GetEndpoints)

							endpointSubRouter.Route("/{endpointID}", func(e chi.Router) {
								e.Use(a.M.RequireEndpoint())

								e.Get("/", a.GetEndpoint)
								e.Put("/", a.UpdateEndpoint)
								e.Delete("/", a.DeleteEndpoint)
								e.Put("/expire_secret", a.ExpireSecret)

								e.Route("/keys", func(keySubRouter chi.Router) {
									keySubRouter.Use(a.M.RequireBaseUrl())
									keySubRouter.With(fflag.CanAccessFeature(fflag.Features[fflag.CanCreateCLIAPIKey])).Post("/", a.CreateEndpointAPIKey)
									keySubRouter.With(a.M.Pagination).Get("/", a.LoadEndpointAPIKeysPaged)
									keySubRouter.Put("/{keyID}/revoke", a.RevokeEndpointAPIKey)
								})

								e.Route("/devices", func(deviceRouter chi.Router) {
									deviceRouter.With(a.M.Pagination).Get("/", a.FindDevicesByAppID)
								})
							})

						})

						groupSubRouter.Route("/events", func(eventRouter chi.Router) {
							eventRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleAdmin))

							eventRouter.Post("/", a.CreateEndpointEvent)
							eventRouter.Post("/fanout", a.CreateEndpointFanoutEvent)
							eventRouter.With(a.M.Pagination).Get("/", a.GetEventsPaged)

							eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
								eventSubRouter.Use(a.M.RequireEvent())
								eventSubRouter.Get("/", a.GetEndpointEvent)
								eventSubRouter.Put("/replay", a.ReplayEndpointEvent)
							})
						})

						groupSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
							eventDeliveryRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

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

						groupSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
							subscriptionRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleAdmin))

							subscriptionRouter.Post("/", a.CreateSubscription)
							subscriptionRouter.Post("/test_filter", a.TestSubscriptionFilter)
							subscriptionRouter.With(a.M.Pagination).Get("/", a.GetSubscriptions)
							subscriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
							subscriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
							subscriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
						})

						groupSubRouter.Route("/sources", func(sourceRouter chi.Router) {
							sourceRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleAdmin))
							sourceRouter.Use(a.M.RequireBaseUrl())

							sourceRouter.Post("/", a.CreateSource)
							sourceRouter.Get("/{sourceID}", a.GetSourceByID)
							sourceRouter.With(a.M.Pagination).Get("/", a.LoadSourcesPaged)
							sourceRouter.Put("/{sourceID}", a.UpdateSource)
							sourceRouter.Delete("/{sourceID}", a.DeleteSource)
						})

						groupSubRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
							dashboardRouter.Get("/summary", a.GetDashboardSummary)
							dashboardRouter.Get("/config", a.GetAllConfigDetails)
						})

						groupSubRouter.Route("/portal-links", func(portalLinkRouter chi.Router) {
							portalLinkRouter.Use(a.M.RequireBaseUrl())

							portalLinkRouter.Post("/", a.CreatePortalLink)
							portalLinkRouter.Get("/{portalLinkID}", a.GetPortalLinkByID)
							portalLinkRouter.With(a.M.Pagination).Get("/", a.LoadPortalLinksPaged)
							portalLinkRouter.Put("/{portalLinkID}", a.UpdatePortalLink)
							portalLinkRouter.Put("/{portalLinkID}/revoke", a.RevokePortalLink)
						})
					})
				})
			})
		})

		uiRouter.Route("/configuration", func(configRouter chi.Router) {
			configRouter.Use(a.M.RequireAuthUserMetadata())

			configRouter.Get("/", a.LoadConfiguration)
			configRouter.Post("/", a.CreateConfiguration)
			configRouter.Put("/", a.UpdateConfiguration)
		})

		uiRouter.Post("/flags", flipt.BatchEvaluate)
	})

	// App Portal API.
	router.Route("/portal", func(portalRouter chi.Router) {
		portalRouter.Use(a.M.JsonResponse)
		portalRouter.Use(a.M.SetupCORS)
		portalRouter.Use(a.M.RequirePortalLink())

		portalRouter.Route("/endpoints", func(endpointRouter chi.Router) {
			endpointRouter.Get("/", a.GetPortalLinkEndpoints)
			endpointRouter.Post("/", a.CreatePortalLinkEndpoint)

			endpointRouter.Route("/{endpointID}", func(endpointSubRouter chi.Router) {
				endpointSubRouter.Use(a.M.RequireEndpoint())
				endpointSubRouter.Use(a.M.RequirePortalLinkEndpoint())
				endpointSubRouter.Use(a.M.RequireBaseUrl())

				endpointSubRouter.Get("/", a.GetEndpoint)
				endpointSubRouter.Put("/", a.UpdateEndpoint)
				endpointSubRouter.With(fflag.CanAccessFeature(fflag.Features[fflag.CanCreateCLIAPIKey])).Post("/keys", a.CreateEndpointAPIKey)
			})
		})

		portalRouter.Route("/devices", func(deviceRouter chi.Router) {
			deviceRouter.With(a.M.Pagination).Get("/", a.GetPortalLinkDevices)
		})

		portalRouter.Route("/keys", func(keySubRouter chi.Router) {
			keySubRouter.Use(a.M.RequireBaseUrl())
			keySubRouter.With(a.M.Pagination).Get("/", a.GetPortalLinkKeys)
			keySubRouter.Put("/{keyID}/revoke", a.RevokeEndpointAPIKey)
		})

		portalRouter.Route("/events", func(eventRouter chi.Router) {
			eventRouter.With(a.M.Pagination).Get("/", a.GetEventsPaged)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Use(a.M.RequireEvent())
				eventSubRouter.Get("/", a.GetEndpointEvent)
				eventSubRouter.Put("/replay", a.ReplayEndpointEvent)
			})
		})

		portalRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
			subscriptionRouter.Post("/", a.CreateSubscription)
			subscriptionRouter.Post("/test_filter", a.TestSubscriptionFilter)
			subscriptionRouter.With(a.M.Pagination).Get("/", a.GetSubscriptions)
			subscriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
			subscriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
			subscriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
		})

		portalRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
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

		portalRouter.Get("/project", a.GetGroup)
		portalRouter.Post("/flags", flipt.BatchEvaluate)
	})

	router.Handle("/queue/monitoring/*", a.A.Queue.(*redisqueue.RedisQueue).Monitor())
	router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))
	router.HandleFunc("/*", reactRootHandler)

	metrics.RegisterQueueMetrics(a.A.Queue)
	prometheus.MustRegister(metrics.RequestDuration())
	a.Router = router
	return router
}
