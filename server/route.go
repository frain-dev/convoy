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

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	limiter "github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/queue"
	"github.com/go-chi/chi/v5/middleware"

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
	router.Use(writeRequestIDHeader)
	router.Use(instrumentRequests(app.tracer))
	router.Use(logHttpRequest(app.logger))

	// Ingestion API
	router.Route("/ingest", func(ingestRouter chi.Router) {

		ingestRouter.Post("/{maskID}", app.IngestEvent)
	})

	// Public API.
	router.Route("/api", func(v1Router chi.Router) {

		v1Router.Route("/v1", func(r chi.Router) {
			r.Use(middleware.AllowContentType("application/json"))
			r.Use(jsonResponse)
			r.Use(requireAuth())

			r.Route("/groups", func(groupRouter chi.Router) {
				groupRouter.Get("/", app.GetGroups)
				//groupRouter.With(requirePermission(auth.RoleSuperUser)).Post("/", app.CreateGroup)

				groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
					groupSubRouter.Use(requireGroup(app.groupRepo, app.cache))
					groupSubRouter.Use(rateLimitByGroupID(app.limiter))

					groupSubRouter.With(requirePermission(auth.RoleAdmin)).Get("/", app.GetGroup)
					groupSubRouter.With(requirePermission(auth.RoleSuperUser)).Put("/", app.UpdateGroup)
					groupSubRouter.With(requirePermission(auth.RoleSuperUser)).Delete("/", app.DeleteGroup)
				})
			})

			r.Route("/applications", func(appRouter chi.Router) {
				appRouter.Use(requireGroup(app.groupRepo, app.cache))
				appRouter.Use(rateLimitByGroupID(app.limiter))
				appRouter.Use(requirePermission(auth.RoleAdmin))

				appRouter.Route("/", func(appSubRouter chi.Router) {
					appSubRouter.Post("/", app.CreateApp)
					appRouter.With(pagination).Get("/", app.GetApps)
				})

				appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
					appSubRouter.Use(requireApp(app.appRepo, app.cache))

					appSubRouter.Get("/", app.GetApp)
					appSubRouter.Put("/", app.UpdateApp)
					appSubRouter.Delete("/", app.DeleteApp)

					appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
						endpointAppSubRouter.Post("/", app.CreateAppEndpoint)
						endpointAppSubRouter.Get("/", app.GetAppEndpoints)

						endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
							e.Use(requireAppEndpoint())

							e.Get("/", app.GetAppEndpoint)
							e.Put("/", app.UpdateAppEndpoint)
							e.Delete("/", app.DeleteAppEndpoint)
						})
					})
				})
			})

			r.Route("/events", func(eventRouter chi.Router) {
				eventRouter.Use(requireGroup(app.groupRepo, app.cache))
				eventRouter.Use(rateLimitByGroupID(app.limiter))
				eventRouter.Use(requirePermission(auth.RoleAdmin))

				eventRouter.With(instrumentPath("/events")).Post("/", app.CreateAppEvent)
				eventRouter.With(pagination).Get("/", app.GetEventsPaged)

				eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
					eventSubRouter.Use(requireEvent(app.eventRepo))
					eventSubRouter.Get("/", app.GetAppEvent)
					eventSubRouter.Put("/replay", app.ReplayAppEvent)
				})
			})

			r.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
				eventDeliveryRouter.Use(requireGroup(app.groupRepo, app.cache))
				eventDeliveryRouter.Use(requirePermission(auth.RoleAdmin))

				eventDeliveryRouter.With(pagination).Get("/", app.GetEventDeliveriesPaged)
				eventDeliveryRouter.Post("/forceresend", app.ForceResendEventDeliveries)
				eventDeliveryRouter.Post("/batchretry", app.BatchRetryEventDelivery)
				eventDeliveryRouter.Get("/countbatchretryevents", app.CountAffectedEventDeliveries)

				eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
					eventDeliverySubRouter.Use(requireEventDelivery(app.eventDeliveryRepo, app.appRepo, app.eventRepo))

					eventDeliverySubRouter.Get("/", app.GetEventDelivery)
					eventDeliverySubRouter.Put("/resend", app.ResendEventDelivery)

					eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
						deliveryRouter.Use(fetchDeliveryAttempts())

						deliveryRouter.Get("/", app.GetDeliveryAttempts)
						deliveryRouter.With(requireDeliveryAttempt()).Get("/{deliveryAttemptID}", app.GetDeliveryAttempt)
					})
				})
			})

			r.Route("/security", func(securityRouter chi.Router) {
				securityRouter.Route("/applications/{appID}/keys", func(securitySubRouter chi.Router) {
					securitySubRouter.Use(requirePermission(auth.RoleAdmin))
					securitySubRouter.Use(requireGroup(app.groupRepo, app.cache))
					securitySubRouter.Use(requireApp(app.appRepo, app.cache))
					securitySubRouter.Use(requireBaseUrl())
					securitySubRouter.Post("/", app.CreateAppPortalAPIKey)
				})
			})

			r.Route("/subscriptions", func(subsriptionRouter chi.Router) {
				subsriptionRouter.Use(requireGroup(app.groupRepo, app.cache))
				subsriptionRouter.Use(rateLimitByGroupID(app.limiter))
				subsriptionRouter.Use(requirePermission(auth.RoleAdmin))

				subsriptionRouter.Post("/", app.CreateSubscription)
				subsriptionRouter.With(pagination).Get("/", app.GetSubscriptions)
				subsriptionRouter.Delete("/{subscriptionID}", app.DeleteSubscription)
				subsriptionRouter.Get("/{subscriptionID}", app.GetSubscription)
				subsriptionRouter.Put("/{subscriptionID}", app.UpdateSubscription)
			})

			r.Route("/sources", func(sourceRouter chi.Router) {
				sourceRouter.Use(requireGroup(app.groupRepo, app.cache))
				sourceRouter.Use(requirePermission(auth.RoleAdmin))
				sourceRouter.Use(requireBaseUrl())

				sourceRouter.Post("/", app.CreateSource)
				sourceRouter.Get("/{sourceID}", app.GetSourceByID)
				sourceRouter.With(pagination).Get("/", app.LoadSourcesPaged)
				sourceRouter.Put("/{sourceID}", app.UpdateSource)
				sourceRouter.Delete("/{sourceID}", app.DeleteSource)
			})
		})
	})

	// UI API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(jsonResponse)
		uiRouter.Use(setupCORS)
		uiRouter.Use(middleware.Maybe(requireAuth(), shouldAuthRoute))

		uiRouter.Post("/organisations/process_invite", app.ProcessOrganisationMemberInvite)
		uiRouter.Get("/users/token", app.FindUserByInviteToken)

		uiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.Post("/login", app.LoginUser)
			authRouter.Post("/token/refresh", app.RefreshToken)
			authRouter.Post("/logout", app.LogoutUser)
		})

		uiRouter.Route("/users", func(userRouter chi.Router) {
			userRouter.Use(requireAuthUserMetadata())

			userRouter.Route("/{userID}", func(userSubRouter chi.Router) {
				userSubRouter.Use(requireAuthorizedUser(app.userRepo))

				userSubRouter.Get("/profile", app.GetUser)
				userSubRouter.Put("/profile", app.UpdateUser)
				userSubRouter.Put("/password", app.UpdatePassword)
			})
		})

		uiRouter.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Use(requireAuthUserMetadata())
			orgRouter.Use(requireBaseUrl())

			orgRouter.Post("/", app.CreateOrganisation)
			orgRouter.With(pagination).Get("/", app.GetOrganisationsPaged)

			orgRouter.Route("/{orgID}", func(orgSubRouter chi.Router) {
				orgSubRouter.Use(requireOrganisation(app.orgRepo))
				orgSubRouter.Use(requireOrganisationMembership(app.orgMemberRepo))

				orgSubRouter.Get("/", app.GetOrganisation)
				orgSubRouter.With(requireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", app.UpdateOrganisation)
				orgSubRouter.With(requireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", app.DeleteOrganisation)
				orgSubRouter.With(requireOrganisationMemberRole(auth.RoleSuperUser)).Post("/invite_user", app.InviteUserToOrganisation)

				orgSubRouter.Route("/members", func(orgMemberRouter chi.Router) {
					orgMemberRouter.Use(requireOrganisationMemberRole(auth.RoleSuperUser))

					orgMemberRouter.With(pagination).Get("/", app.GetOrganisationMembers)

					orgMemberRouter.Route("/{memberID}", func(orgMemberSubRouter chi.Router) {

						orgMemberSubRouter.Get("/", app.GetOrganisationMember)
						orgMemberSubRouter.Put("/", app.UpdateOrganisationMember)
						orgMemberSubRouter.Delete("/", app.DeleteOrganisationMember)

					})
				})

				orgSubRouter.Route("/security", func(securityRouter chi.Router) {
					securityRouter.Use(requireOrganisationMemberRole(auth.RoleSuperUser))

					securityRouter.Post("/keys", app.CreateAPIKey)
					securityRouter.With(pagination).Get("/keys", app.GetAPIKeys)
					securityRouter.Get("/keys/{keyID}", app.GetAPIKeyByID)
					securityRouter.Put("/keys/{keyID}", app.UpdateAPIKey)
					securityRouter.Put("/keys/{keyID}/revoke", app.RevokeAPIKey)
				})

				orgSubRouter.Route("/groups", func(groupRouter chi.Router) {
					groupRouter.Route("/", func(orgSubRouter chi.Router) {
						groupRouter.With(requireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", app.CreateGroup)
						groupRouter.Get("/", app.GetGroups)
					})

					groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
						groupSubRouter.Use(requireGroup(app.groupRepo, app.cache))
						groupSubRouter.Use(rateLimitByGroupID(app.limiter))
						// TODO(dotun): uncomment this when a group is linked to an organisation
						groupSubRouter.Use(requireOrganisationGroupMember())

						groupSubRouter.With(requireOrganisationMemberRole(auth.RoleUIAdmin)).Get("/", app.GetGroup)
						groupSubRouter.With(requireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", app.UpdateGroup)
						groupSubRouter.With(requireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", app.DeleteGroup)

						groupSubRouter.Route("/apps", func(appRouter chi.Router) {
							appRouter.Use(requireOrganisationMemberRole(auth.RoleUIAdmin))

							appRouter.Route("/", func(appSubRouter chi.Router) {
								appSubRouter.Post("/", app.CreateApp)
								appRouter.With(pagination).Get("/", app.GetApps)
							})

							appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
								appSubRouter.Use(requireApp(app.appRepo, app.cache))
								appSubRouter.Get("/", app.GetApp)
								appSubRouter.Put("/", app.UpdateApp)
								appSubRouter.Delete("/", app.DeleteApp)

								appSubRouter.Route("/keys", func(keySubRouter chi.Router) {
									keySubRouter.Use(requireBaseUrl())
									keySubRouter.Post("/", app.CreateAppPortalAPIKey)
								})

								appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
									endpointAppSubRouter.Post("/", app.CreateAppEndpoint)
									endpointAppSubRouter.Get("/", app.GetAppEndpoints)

									endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
										e.Use(requireAppEndpoint())

										e.Get("/", app.GetAppEndpoint)
										e.Put("/", app.UpdateAppEndpoint)
										e.Delete("/", app.DeleteAppEndpoint)
									})
								})
							})
						})

						groupSubRouter.Route("/events", func(eventRouter chi.Router) {
							eventRouter.Use(requireOrganisationMemberRole(auth.RoleUIAdmin))

							eventRouter.Post("/", app.CreateAppEvent)
							eventRouter.With(pagination).Get("/", app.GetEventsPaged)

							eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
								eventSubRouter.Use(requireEvent(app.eventRepo))
								eventSubRouter.Get("/", app.GetAppEvent)
								eventSubRouter.Put("/replay", app.ReplayAppEvent)
							})
						})

						groupSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
							eventDeliveryRouter.Use(requireOrganisationMemberRole(auth.RoleUIAdmin))

							eventDeliveryRouter.With(pagination).Get("/", app.GetEventDeliveriesPaged)
							eventDeliveryRouter.Post("/forceresend", app.ForceResendEventDeliveries)
							eventDeliveryRouter.Post("/batchretry", app.BatchRetryEventDelivery)
							eventDeliveryRouter.Get("/countbatchretryevents", app.CountAffectedEventDeliveries)

							eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
								eventDeliverySubRouter.Use(requireEventDelivery(app.eventDeliveryRepo, app.appRepo, app.eventRepo))

								eventDeliverySubRouter.Get("/", app.GetEventDelivery)
								eventDeliverySubRouter.Put("/resend", app.ResendEventDelivery)

								eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
									deliveryRouter.Use(fetchDeliveryAttempts())

									deliveryRouter.Get("/", app.GetDeliveryAttempts)
									deliveryRouter.With(requireDeliveryAttempt()).Get("/{deliveryAttemptID}", app.GetDeliveryAttempt)
								})
							})
						})

						groupSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
							subscriptionRouter.Use(requireOrganisationMemberRole(auth.RoleAdmin))

							subscriptionRouter.Post("/", app.CreateSubscription)
							subscriptionRouter.With(pagination).Get("/", app.GetSubscriptions)
							subscriptionRouter.Delete("/", app.DeleteSubscription)
							subscriptionRouter.Get("/{subscriptionID}", app.GetSubscription)
							subscriptionRouter.Put("/{subscriptionID}", app.UpdateSubscription)
						})

						groupSubRouter.Route("/sources", func(sourceRouter chi.Router) {
							sourceRouter.Use(requireOrganisationMemberRole(auth.RoleAdmin))
							sourceRouter.Use(requireBaseUrl())

							sourceRouter.Post("/", app.CreateSource)
							sourceRouter.Get("/{sourceID}", app.GetSourceByID)
							sourceRouter.With(pagination).Get("/", app.LoadSourcesPaged)
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
			configRouter.Use(requirePermission(auth.RoleAdmin))

			configRouter.Get("/", app.LoadConfiguration)
			configRouter.Post("/", app.CreateConfiguration)
		})
	})

	//App Portal API.
	router.Route("/portal", func(portalRouter chi.Router) {
		portalRouter.Use(jsonResponse)
		portalRouter.Use(setupCORS)
		portalRouter.Use(requireAuth())
		portalRouter.Use(requireGroup(app.groupRepo, app.cache))
		portalRouter.Use(requireAppID())

		portalRouter.Route("/apps", func(appRouter chi.Router) {
			appRouter.Use(requireAppPortalApplication(app.appRepo))
			appRouter.Use(requireAppPortalPermission(auth.RoleUIAdmin))

			appRouter.Get("/", app.GetApp)

			appRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
				endpointAppSubRouter.Get("/", app.GetAppEndpoints)
				endpointAppSubRouter.Post("/", app.CreateAppEndpoint)

				endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
					e.Use(requireAppEndpoint())

					e.Get("/", app.GetAppEndpoint)
					e.Put("/", app.UpdateAppEndpoint)
				})
			})
		})

		portalRouter.Route("/events", func(eventRouter chi.Router) {
			eventRouter.Use(requireAppPortalApplication(app.appRepo))
			eventRouter.Use(requireAppPortalPermission(auth.RoleUIAdmin))

			eventRouter.With(pagination).Get("/", app.GetEventsPaged)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Use(requireEvent(app.eventRepo))
				eventSubRouter.Get("/", app.GetAppEvent)
				eventSubRouter.Put("/replay", app.ReplayAppEvent)
			})
		})

		portalRouter.Route("/subscriptions", func(subsriptionRouter chi.Router) {
			subsriptionRouter.Use(requireAppPortalApplication(app.appRepo))
			subsriptionRouter.Use(requireAppPortalPermission(auth.RoleUIAdmin))

			subsriptionRouter.Post("/", app.CreateSubscription)
			subsriptionRouter.With(pagination).Get("/", app.GetSubscriptions)
			subsriptionRouter.Delete("/{subscriptionID}", app.DeleteSubscription)
			subsriptionRouter.Get("/{subscriptionID}", app.GetSubscription)
			subsriptionRouter.Put("/{subscriptionID}", app.UpdateSubscription)
		})

		portalRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
			eventDeliveryRouter.Use(requireAppPortalApplication(app.appRepo))
			eventDeliveryRouter.Use(requireAppPortalPermission(auth.RoleUIAdmin))

			eventDeliveryRouter.With(pagination).Get("/", app.GetEventDeliveriesPaged)
			eventDeliveryRouter.Post("/forceresend", app.ForceResendEventDeliveries)
			eventDeliveryRouter.Post("/batchretry", app.BatchRetryEventDelivery)
			eventDeliveryRouter.Get("/countbatchretryevents", app.CountAffectedEventDeliveries)

			eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
				eventDeliverySubRouter.Use(requireEventDelivery(app.eventDeliveryRepo, app.appRepo, app.eventRepo))

				eventDeliverySubRouter.Get("/", app.GetEventDelivery)
				eventDeliverySubRouter.Put("/resend", app.ResendEventDelivery)

				eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
					deliveryRouter.Use(fetchDeliveryAttempts())

					deliveryRouter.Get("/", app.GetDeliveryAttempts)
					deliveryRouter.With(requireDeliveryAttempt()).Get("/{deliveryAttemptID}", app.GetDeliveryAttempt)
				})
			})
		})
	})

	router.Handle("/queue/monitoring/*", app.queue.(*redisqueue.RedisQueue).Monitor())
	router.Handle("/metrics", promhttp.HandlerFor(Reg, promhttp.HandlerOpts{}))
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = render.Render(w, r, newServerResponse(fmt.Sprintf("Convoy %v", convoy.GetVersion()), nil, http.StatusOK))
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

	RegisterQueueMetrics(app.queue, cfg)
	RegisterDBMetrics(app)
	prometheus.MustRegister(requestDuration)
	return srv
}
