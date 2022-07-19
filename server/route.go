package server

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/logger"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/searcher"
	"github.com/frain-dev/convoy/tracer"
	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	limiter "github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/queue"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/frain-dev/convoy/internal/pkg/metrics"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"

	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/go-chi/chi/v5"
)

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
		log.WithError(err).Error("an error has occurred with the react app")
		return
	}
	if _, err := static.Open(strings.TrimLeft(p, "/")); err != nil { // If file not found server index/html from root
		req.URL.Path = "/"
	}
	http.FileServer(http.FS(static)).ServeHTTP(rw, req)
}

func buildRoutes(app *applicationHandler) http.Handler {

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(m.WriteRequestIDHeader)
	router.Use(m.InstrumentRequests(app.tracer))
	router.Use(m.LogHttpRequest(app.logger))

	// Ingestion API
	router.Route("/ingest", func(ingestRouter chi.Router) {

		ingestRouter.Get("/{maskID}", app.HandleCrcCheck)
		ingestRouter.Post("/{maskID}", app.IngestEvent)
	})

	// Public API.
	router.Route("/api", func(v1Router chi.Router) {

		v1Router.Route("/v1", func(r chi.Router) {
			r.Use(middleware.AllowContentType("application/json"))
			r.Use(m.JsonResponse)
			r.Use(m.RequireAuth())

			r.Route("/applications", func(appRouter chi.Router) {
				appRouter.Use(m.RequireGroup(app.groupRepo, app.cache))
				appRouter.Use(m.RateLimitByGroupID(app.limiter))
				appRouter.Use(m.RequirePermission(auth.RoleAdmin))

				appRouter.Route("/", func(appSubRouter chi.Router) {
					appSubRouter.Post("/", app.CreateApp)
					appRouter.With(m.Pagination).Get("/", app.GetApps)
				})

				appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
					appSubRouter.Use(m.RequireApp(app.appRepo, app.cache))

					appSubRouter.Get("/", app.GetApp)
					appSubRouter.Put("/", app.UpdateApp)
					appSubRouter.Delete("/", app.DeleteApp)

					appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
						endpointAppSubRouter.Post("/", app.CreateAppEndpoint)
						endpointAppSubRouter.Get("/", app.GetAppEndpoints)

						endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
							e.Use(m.RequireAppEndpoint())

							e.Get("/", app.GetAppEndpoint)
							e.Put("/", app.UpdateAppEndpoint)
							e.Delete("/", app.DeleteAppEndpoint)
						})
					})
				})
			})

			r.Route("/events", func(eventRouter chi.Router) {
				eventRouter.Use(m.RequireGroup(app.groupRepo, app.cache))
				eventRouter.Use(m.RateLimitByGroupID(app.limiter))
				eventRouter.Use(m.RequirePermission(auth.RoleAdmin))

				eventRouter.With(m.InstrumentPath("/events")).Post("/", app.CreateAppEvent)
				eventRouter.With(m.Pagination).Get("/", app.GetEventsPaged)

				eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
					eventSubRouter.Use(m.RequireEvent(app.eventRepo))
					eventSubRouter.Get("/", app.GetAppEvent)
					eventSubRouter.Put("/replay", app.ReplayAppEvent)
				})
			})

			r.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
				eventDeliveryRouter.Use(m.RequireGroup(app.groupRepo, app.cache))
				eventDeliveryRouter.Use(m.RequirePermission(auth.RoleAdmin))

				eventDeliveryRouter.With(m.Pagination).Get("/", app.GetEventDeliveriesPaged)
				eventDeliveryRouter.Post("/forceresend", app.ForceResendEventDeliveries)
				eventDeliveryRouter.Post("/batchretry", app.BatchRetryEventDelivery)
				eventDeliveryRouter.Get("/countbatchretryevents", app.CountAffectedEventDeliveries)

				eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
					eventDeliverySubRouter.Use(m.RequireEventDelivery(app.eventDeliveryRepo, app.appRepo, app.eventRepo))

					eventDeliverySubRouter.Get("/", app.GetEventDelivery)
					eventDeliverySubRouter.Put("/resend", app.ResendEventDelivery)

					eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
						deliveryRouter.Use(fetchDeliveryAttempts())

						deliveryRouter.Get("/", app.GetDeliveryAttempts)
						deliveryRouter.With(m.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", app.GetDeliveryAttempt)
					})
				})
			})

			r.Route("/security", func(securityRouter chi.Router) {
				securityRouter.Route("/applications/{appID}/keys", func(securitySubRouter chi.Router) {
					securitySubRouter.Use(m.RequireGroup(app.groupRepo, app.cache))
					securitySubRouter.Use(m.RequirePermission(auth.RoleAdmin))
					securitySubRouter.Use(m.RequireApp(app.appRepo, app.cache))
					securitySubRouter.Use(m.RequireBaseUrl())
					securitySubRouter.Post("/", app.CreateAppPortalAPIKey)
				})
			})

			r.Route("/subscriptions", func(subsriptionRouter chi.Router) {
				subsriptionRouter.Use(m.RequireGroup(app.groupRepo, app.cache))
				subsriptionRouter.Use(m.RateLimitByGroupID(app.limiter))
				subsriptionRouter.Use(m.RequirePermission(auth.RoleAdmin))

				subsriptionRouter.Post("/", app.CreateSubscription)
				subsriptionRouter.With(m.Pagination).Get("/", app.GetSubscriptions)
				subsriptionRouter.Delete("/{subscriptionID}", app.DeleteSubscription)
				subsriptionRouter.Get("/{subscriptionID}", app.GetSubscription)
				subsriptionRouter.Put("/{subscriptionID}", app.UpdateSubscription)
				subsriptionRouter.Put("/{subscriptionID}/toggle_status", app.ToggleSubscriptionStatus)
			})

			r.Route("/sources", func(sourceRouter chi.Router) {
				sourceRouter.Use(m.RequireGroup(app.groupRepo, app.cache))
				sourceRouter.Use(m.RequirePermission(auth.RoleAdmin))
				sourceRouter.Use(m.RequireBaseUrl())

				sourceRouter.Post("/", app.CreateSource)
				sourceRouter.Get("/{sourceID}", app.GetSourceByID)
				sourceRouter.With(m.Pagination).Get("/", app.LoadSourcesPaged)
				sourceRouter.Put("/{sourceID}", app.UpdateSource)
				sourceRouter.Delete("/{sourceID}", app.DeleteSource)
			})
		})
	})

	// UI API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(m.JsonResponse)
		uiRouter.Use(m.SetupCORS)
		uiRouter.Use(middleware.Maybe(m.RequireAuth(), m.ShouldAuthRoute))
		uiRouter.Use(m.RequireBaseUrl())

		uiRouter.Post("/organisations/process_invite", app.ProcessOrganisationMemberInvite)
		uiRouter.Get("/users/token", app.FindUserByInviteToken)

		uiRouter.Route("/users", func(userRouter chi.Router) {
			userRouter.Use(m.RequireAuthUserMetadata())
			userRouter.Route("/{userID}", func(userSubRouter chi.Router) {
				userSubRouter.Use(m.RequireAuthorizedUser(app.userRepo))
				userSubRouter.Get("/profile", app.GetUser)
				userSubRouter.Put("/profile", app.UpdateUser)
				userSubRouter.Put("/password", app.UpdatePassword)
			})
		})

		uiRouter.Post("/users/forgot-password", app.ForgotPassword)
		uiRouter.Post("/users/reset-password", app.ResetPassword)

		uiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.Post("/login", app.LoginUser)
			authRouter.Post("/token/refresh", app.RefreshToken)
			authRouter.Post("/logout", app.LogoutUser)
		})

		uiRouter.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Use(m.RequireAuthUserMetadata())
			orgRouter.Use(m.RequireBaseUrl())

			orgRouter.Post("/", app.CreateOrganisation)
			orgRouter.With(m.Pagination).Get("/", app.GetOrganisationsPaged)

			orgRouter.Route("/{orgID}", func(orgSubRouter chi.Router) {
				orgSubRouter.Use(m.RequireOrganisation(app.orgRepo))
				orgSubRouter.Use(m.RequireOrganisationMembership(app.orgMemberRepo))

				orgSubRouter.Get("/", app.GetOrganisation)
				orgSubRouter.With(m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", app.UpdateOrganisation)
				orgSubRouter.With(m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", app.DeleteOrganisation)

				orgSubRouter.Route("/invites", func(orgInvitesRouter chi.Router) {
					orgInvitesRouter.With(m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", app.InviteUserToOrganisation)
					orgInvitesRouter.With(m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/resend", app.ResendOrganizationInvite)
					orgInvitesRouter.With(m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/cancel", app.CancelOrganizationInvite)
					orgInvitesRouter.With(m.RequireOrganisationMemberRole(auth.RoleSuperUser)).With(m.Pagination).Get("/pending", app.GetPendingOrganisationInvites)
				})

				orgSubRouter.Route("/members", func(orgMemberRouter chi.Router) {
					orgMemberRouter.Use(m.RequireOrganisationMemberRole(auth.RoleSuperUser))

					orgMemberRouter.With(m.Pagination).Get("/", app.GetOrganisationMembers)

					orgMemberRouter.Route("/{memberID}", func(orgMemberSubRouter chi.Router) {

						orgMemberSubRouter.Get("/", app.GetOrganisationMember)
						orgMemberSubRouter.Put("/", app.UpdateOrganisationMember)
						orgMemberSubRouter.Delete("/", app.DeleteOrganisationMember)

					})
				})

				orgSubRouter.Route("/security", func(securityRouter chi.Router) {
					securityRouter.Use(m.RequireOrganisationMemberRole(auth.RoleSuperUser))

					securityRouter.Post("/keys", app.CreateAPIKey)
					securityRouter.With(m.Pagination).Get("/keys", app.GetAPIKeys)
					securityRouter.Get("/keys/{keyID}", app.GetAPIKeyByID)
					securityRouter.Put("/keys/{keyID}", app.UpdateAPIKey)
					securityRouter.Put("/keys/{keyID}/revoke", app.RevokeAPIKey)
				})

				orgSubRouter.Route("/groups", func(groupRouter chi.Router) {
					groupRouter.Route("/", func(orgSubRouter chi.Router) {
						groupRouter.With(m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", app.CreateGroup)
						groupRouter.Get("/", app.GetGroups)
					})

					groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
						groupSubRouter.Use(m.RequireGroup(app.groupRepo, app.cache))
						groupSubRouter.Use(m.RateLimitByGroupID(app.limiter))
						groupSubRouter.Use(m.RequireOrganisationGroupMember())

						groupSubRouter.With(m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Get("/", app.GetGroup)
						groupSubRouter.With(m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", app.UpdateGroup)
						groupSubRouter.With(m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", app.DeleteGroup)

						groupSubRouter.Route("/apps", func(appRouter chi.Router) {
							appRouter.Use(m.RequireOrganisationMemberRole(auth.RoleSuperUser))

							appRouter.Route("/", func(appSubRouter chi.Router) {
								appSubRouter.Post("/", app.CreateApp)
								appRouter.With(m.Pagination).Get("/", app.GetApps)
							})

							appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
								appSubRouter.Use(m.RequireApp(app.appRepo, app.cache))
								appSubRouter.Get("/", app.GetApp)
								appSubRouter.Put("/", app.UpdateApp)
								appSubRouter.Delete("/", app.DeleteApp)

								appSubRouter.Route("/keys", func(keySubRouter chi.Router) {
									keySubRouter.Use(m.RequireBaseUrl())
									keySubRouter.Post("/", app.CreateAppPortalAPIKey)
								})

								appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
									endpointAppSubRouter.Post("/", app.CreateAppEndpoint)
									endpointAppSubRouter.Get("/", app.GetAppEndpoints)

									endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
										e.Use(m.RequireAppEndpoint())

										e.Get("/", app.GetAppEndpoint)
										e.Put("/", app.UpdateAppEndpoint)
										e.Delete("/", app.DeleteAppEndpoint)
									})
								})
							})
						})

						groupSubRouter.Route("/events", func(eventRouter chi.Router) {
							eventRouter.Use(m.RequireOrganisationMemberRole(auth.RoleAdmin))

							eventRouter.Post("/", app.CreateAppEvent)
							eventRouter.With(m.Pagination).Get("/", app.GetEventsPaged)

							eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
								eventSubRouter.Use(m.RequireEvent(app.eventRepo))
								eventSubRouter.Get("/", app.GetAppEvent)
								eventSubRouter.Put("/replay", app.ReplayAppEvent)
							})
						})

						groupSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
							eventDeliveryRouter.Use(m.RequireOrganisationMemberRole(auth.RoleSuperUser))

							eventDeliveryRouter.With(m.Pagination).Get("/", app.GetEventDeliveriesPaged)
							eventDeliveryRouter.Post("/forceresend", app.ForceResendEventDeliveries)
							eventDeliveryRouter.Post("/batchretry", app.BatchRetryEventDelivery)
							eventDeliveryRouter.Get("/countbatchretryevents", app.CountAffectedEventDeliveries)

							eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
								eventDeliverySubRouter.Use(m.RequireEventDelivery(app.eventDeliveryRepo, app.appRepo, app.eventRepo))

								eventDeliverySubRouter.Get("/", app.GetEventDelivery)
								eventDeliverySubRouter.Put("/resend", app.ResendEventDelivery)

								eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
									deliveryRouter.Use(fetchDeliveryAttempts())

									deliveryRouter.Get("/", app.GetDeliveryAttempts)
									deliveryRouter.With(m.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", app.GetDeliveryAttempt)
								})
							})
						})

						groupSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
							subscriptionRouter.Use(m.RequireOrganisationMemberRole(auth.RoleAdmin))

							subscriptionRouter.Post("/", app.CreateSubscription)
							subscriptionRouter.With(m.Pagination).Get("/", app.GetSubscriptions)
							subscriptionRouter.Delete("/{subscriptionID}", app.DeleteSubscription)
							subscriptionRouter.Get("/{subscriptionID}", app.GetSubscription)
							subscriptionRouter.Put("/{subscriptionID}", app.UpdateSubscription)
						})

						groupSubRouter.Route("/sources", func(sourceRouter chi.Router) {
							sourceRouter.Use(m.RequireOrganisationMemberRole(auth.RoleAdmin))
							sourceRouter.Use(m.RequireBaseUrl())

							sourceRouter.Post("/", app.CreateSource)
							sourceRouter.Get("/{sourceID}", app.GetSourceByID)
							sourceRouter.With(m.Pagination).Get("/", app.LoadSourcesPaged)
							sourceRouter.Put("/{sourceID}", app.UpdateSource)
							sourceRouter.Delete("/{sourceID}", app.DeleteSource)
						})

						groupSubRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
							dashboardRouter.Get("/summary", app.GetDashboardSummary)
							dashboardRouter.Get("/config", app.GetAllConfigDetails)
						})
					})

				})
			})
		})

		uiRouter.Route("/configuration", func(configRouter chi.Router) {
			configRouter.Use(m.RequireAuthUserMetadata())

			configRouter.Get("/", app.LoadConfiguration)
			configRouter.Post("/", app.CreateConfiguration)
			configRouter.Put("/", app.UpdateConfiguration)

		})
	})

	//App Portal API.
	router.Route("/portal", func(portalRouter chi.Router) {
		portalRouter.Use(m.JsonResponse)
		portalRouter.Use(m.SetupCORS)
		portalRouter.Use(m.RequireAuth())
		portalRouter.Use(m.RequireGroup(app.groupRepo, app.cache))
		portalRouter.Use(m.RequireAppID())

		portalRouter.Route("/apps", func(appRouter chi.Router) {
			appRouter.Use(m.RequireAppPortalApplication(app.appRepo))
			appRouter.Use(m.RequireAppPortalPermission(auth.RoleAdmin))

			appRouter.Get("/", app.GetApp)

			appRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
				endpointAppSubRouter.Get("/", app.GetAppEndpoints)
				endpointAppSubRouter.Post("/", app.CreateAppEndpoint)

				endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
					e.Use(m.RequireAppEndpoint())

					e.Get("/", app.GetAppEndpoint)
					e.Put("/", app.UpdateAppEndpoint)
				})
			})
		})

		portalRouter.Route("/events", func(eventRouter chi.Router) {
			eventRouter.Use(m.RequireAppPortalApplication(app.appRepo))
			eventRouter.Use(m.RequireAppPortalPermission(auth.RoleAdmin))

			eventRouter.With(m.Pagination).Get("/", app.GetEventsPaged)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Use(m.RequireEvent(app.eventRepo))
				eventSubRouter.Get("/", app.GetAppEvent)
				eventSubRouter.Put("/replay", app.ReplayAppEvent)
			})
		})

		portalRouter.Route("/subscriptions", func(subsriptionRouter chi.Router) {
			subsriptionRouter.Use(m.RequireAppPortalApplication(app.appRepo))
			subsriptionRouter.Use(m.RequireAppPortalPermission(auth.RoleAdmin))

			subsriptionRouter.Post("/", app.CreateSubscription)
			subsriptionRouter.With(m.Pagination).Get("/", app.GetSubscriptions)
			subsriptionRouter.Delete("/{subscriptionID}", app.DeleteSubscription)
			subsriptionRouter.Get("/{subscriptionID}", app.GetSubscription)
			subsriptionRouter.Put("/{subscriptionID}", app.UpdateSubscription)
		})

		portalRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
			eventDeliveryRouter.Use(m.RequireAppPortalApplication(app.appRepo))
			eventDeliveryRouter.Use(m.RequireAppPortalPermission(auth.RoleAdmin))

			eventDeliveryRouter.With(m.Pagination).Get("/", app.GetEventDeliveriesPaged)
			eventDeliveryRouter.Post("/forceresend", app.ForceResendEventDeliveries)
			eventDeliveryRouter.Post("/batchretry", app.BatchRetryEventDelivery)
			eventDeliveryRouter.Get("/countbatchretryevents", app.CountAffectedEventDeliveries)

			eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
				eventDeliverySubRouter.Use(m.RequireEventDelivery(app.eventDeliveryRepo, app.appRepo, app.eventRepo))

				eventDeliverySubRouter.Get("/", app.GetEventDelivery)
				eventDeliverySubRouter.Put("/resend", app.ResendEventDelivery)

				eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
					deliveryRouter.Use(fetchDeliveryAttempts())

					deliveryRouter.Get("/", app.GetDeliveryAttempts)
					deliveryRouter.With(m.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", app.GetDeliveryAttempt)
				})
			})
		})
	})

	router.Handle("/queue/monitoring/*", app.queue.(*redisqueue.RedisQueue).Monitor())
	router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("Convoy %v", convoy.GetVersion()), nil, http.StatusOK))
	})
	router.HandleFunc("/*", reactRootHandler)

	return router
}

func New(cfg config.Configuration,
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	appRepo datastore.ApplicationRepository,
	apiKeyRepo datastore.APIKeyRepository,
	subRepo datastore.SubscriptionRepository,
	groupRepo datastore.GroupRepository,
	orgRepo datastore.OrganisationRepository,
	orgMemberRepo datastore.OrganisationMemberRepository,
	orgInviteRepo datastore.OrganisationInviteRepository,
	sourceRepo datastore.SourceRepository,
	userRepo datastore.UserRepository,
	configRepo datastore.ConfigurationRepository,
	queue queue.Queuer,
	logger logger.Logger,
	tracer tracer.Tracer,
	cache cache.Cache,
	limiter limiter.RateLimiter,
	searcher searcher.Searcher,
) *http.Server {

	app := newApplicationHandler(
		eventRepo,
		eventDeliveryRepo,
		appRepo,
		groupRepo,
		apiKeyRepo,
		subRepo,
		sourceRepo,
		orgRepo,
		orgMemberRepo,
		orgInviteRepo,
		userRepo,
		configRepo,
		queue,
		logger,
		tracer,
		cache,
		limiter,
		searcher,
	)

	srv := &http.Server{
		Handler:      buildRoutes(app),
		ReadTimeout:  time.Second * 30,
		WriteTimeout: time.Second * 30,
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
	}

	metrics.RegisterQueueMetrics(app.queue, cfg)
	metrics.RegisterDBMetrics(app.eventDeliveryRepo)
	prometheus.MustRegister(metrics.RequestDuration())
	return srv
}
