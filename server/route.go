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
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/logger"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/searcher"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/tracer"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

type ApplicationHandler struct {
	M *middleware.Middleware
	S Services
	R Repos
}

type Repos struct {
	EventRepo         datastore.EventRepository
	EventDeliveryRepo datastore.EventDeliveryRepository
	AppRepo           datastore.ApplicationRepository
	GroupRepo         datastore.GroupRepository
	ApiKeyRepo        datastore.APIKeyRepository
	SubRepo           datastore.SubscriptionRepository
	SourceRepo        datastore.SourceRepository
	OrgRepo           datastore.OrganisationRepository
	OrgMemberRepo     datastore.OrganisationMemberRepository
	OrgInviteRepo     datastore.OrganisationInviteRepository
	UserRepo          datastore.UserRepository
	ConfigRepo        datastore.ConfigurationRepository
	DeviceRepo        datastore.DeviceRepository
}

type Services struct {
	Queue    queue.Queuer
	Logger   logger.Logger
	Tracer   tracer.Tracer
	Cache    cache.Cache
	Limiter  limiter.RateLimiter
	Searcher searcher.Searcher

	AppService                *services.AppService
	EventService              *services.EventService
	GroupService              *services.GroupService
	SecurityService           *services.SecurityService
	SourceService             *services.SourceService
	ConfigService             *services.ConfigService
	UserService               *services.UserService
	SubService                *services.SubcriptionService
	OrganisationService       *services.OrganisationService
	OrganisationMemberService *services.OrganisationMemberService
	OrganisationInviteService *services.OrganisationInviteService
	DeviceService             *services.DeviceService
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
		log.WithError(err).Error("an error has occurred with the react app")
		return
	}
	if _, err := static.Open(strings.TrimLeft(p, "/")); err != nil { // If file not found server index/html from root
		req.URL.Path = "/"
	}
	http.FileServer(http.FS(static)).ServeHTTP(rw, req)
}

func NewApplicationHandler(r Repos, s Services) *ApplicationHandler {
	as := services.NewAppService(r.AppRepo, r.EventRepo, r.EventDeliveryRepo, s.Cache)
	es := services.NewEventService(r.AppRepo, r.EventRepo, r.EventDeliveryRepo, s.Queue, s.Cache, s.Searcher, r.SubRepo)
	gs := services.NewGroupService(r.ApiKeyRepo, r.AppRepo, r.GroupRepo, r.EventRepo, r.EventDeliveryRepo, s.Limiter, s.Cache)
	ss := services.NewSecurityService(r.GroupRepo, r.ApiKeyRepo)
	os := services.NewOrganisationService(r.OrgRepo, r.OrgMemberRepo)
	rs := services.NewSubscriptionService(r.SubRepo, r.AppRepo, r.SourceRepo)
	sos := services.NewSourceService(r.SourceRepo, s.Cache)
	us := services.NewUserService(r.UserRepo, s.Cache, s.Queue)
	ois := services.NewOrganisationInviteService(r.OrgRepo, r.UserRepo, r.OrgMemberRepo, r.OrgInviteRepo, s.Queue)
	om := services.NewOrganisationMemberService(r.OrgMemberRepo)
	cs := services.NewConfigService(r.ConfigRepo)
	ds := services.NewDeviceService(r.DeviceRepo)

	m := middleware.NewMiddleware(&middleware.CreateMiddleware{
		EventRepo:         r.EventRepo,
		EventDeliveryRepo: r.EventDeliveryRepo,
		AppRepo:           r.AppRepo,
		GroupRepo:         r.GroupRepo,
		ApiKeyRepo:        r.ApiKeyRepo,
		SubRepo:           r.SubRepo,
		SourceRepo:        r.SourceRepo,
		OrgRepo:           r.OrgRepo,
		OrgMemberRepo:     r.OrgMemberRepo,
		OrgInviteRepo:     r.OrgInviteRepo,
		UserRepo:          r.UserRepo,
		ConfigRepo:        r.ConfigRepo,
		Cache:             s.Cache,
		Logger:            s.Logger,
		Limiter:           s.Limiter,
		Tracer:            s.Tracer,
	})

	return &ApplicationHandler{
		M: m,
		R: Repos{
			EventRepo:         r.EventRepo,
			EventDeliveryRepo: r.EventDeliveryRepo,
			AppRepo:           r.AppRepo,
			GroupRepo:         r.GroupRepo,
			ApiKeyRepo:        r.ApiKeyRepo,
			SubRepo:           r.SubRepo,
			SourceRepo:        r.SourceRepo,
			OrgRepo:           r.OrgRepo,
			OrgMemberRepo:     r.OrgMemberRepo,
			OrgInviteRepo:     r.OrgInviteRepo,
			UserRepo:          r.UserRepo,
			ConfigRepo:        r.ConfigRepo,
			DeviceRepo:        r.DeviceRepo,
		},
		S: Services{
			Queue:                     s.Queue,
			Cache:                     s.Cache,
			Searcher:                  s.Searcher,
			Logger:                    s.Logger,
			Tracer:                    s.Tracer,
			Limiter:                   s.Limiter,
			AppService:                as,
			EventService:              es,
			GroupService:              gs,
			SecurityService:           ss,
			SourceService:             sos,
			ConfigService:             cs,
			UserService:               us,
			SubService:                rs,
			OrganisationService:       os,
			OrganisationMemberService: om,
			OrganisationInviteService: ois,
			DeviceService:             ds,
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

			r.Route("/applications", func(appRouter chi.Router) {
				appRouter.Use(a.M.RequireGroup())
				appRouter.Use(a.M.RateLimitByGroupID())
				appRouter.Use(a.M.RequirePermission(auth.RoleAdmin))

				appRouter.Route("/", func(appSubRouter chi.Router) {
					appSubRouter.Post("/", a.CreateApp)
					appRouter.With(a.M.Pagination).Get("/", a.GetApps)
				})

				appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
					appSubRouter.Use(a.M.RequireApp())

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
						})
					})
				})
			})

			r.Route("/events", func(eventRouter chi.Router) {
				eventRouter.Use(a.M.RequireGroup())
				eventRouter.Use(a.M.RateLimitByGroupID())
				eventRouter.Use(a.M.RequirePermission(auth.RoleAdmin))

				eventRouter.With(a.M.InstrumentPath("/events")).Post("/", a.CreateAppEvent)
				eventRouter.With(a.M.Pagination).Get("/", a.GetEventsPaged)

				eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
					eventSubRouter.Use(a.M.RequireEvent())
					eventSubRouter.Get("/", a.GetAppEvent)
					eventSubRouter.Put("/replay", a.ReplayAppEvent)
				})
			})

			r.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
				eventDeliveryRouter.Use(a.M.RequireGroup())
				eventDeliveryRouter.Use(a.M.RequirePermission(auth.RoleAdmin))

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

			r.Route("/security", func(securityRouter chi.Router) {
				securityRouter.Route("/applications/{appID}/keys", func(securitySubRouter chi.Router) {
					securitySubRouter.Use(a.M.RequireGroup())
					securitySubRouter.Use(a.M.RequirePermission(auth.RoleAdmin))
					securitySubRouter.Use(a.M.RequireApp())
					securitySubRouter.Use(a.M.RequireBaseUrl())
					securitySubRouter.Post("/", a.CreateAppPortalAPIKey)
				})
			})

			r.Route("/subscriptions", func(subscriptionRouter chi.Router) {
				subscriptionRouter.Use(a.M.RequireGroup())
				subscriptionRouter.Use(a.M.RateLimitByGroupID())
				subscriptionRouter.Use(a.M.RequirePermission(auth.RoleAdmin))

				subscriptionRouter.Post("/", a.CreateSubscription)
				subscriptionRouter.With(a.M.Pagination).Get("/", a.GetSubscriptions)
				subscriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
				subscriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
				subscriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
				subscriptionRouter.Put("/{subscriptionID}/toggle_status", a.ToggleSubscriptionStatus)
			})

			r.Route("/sources", func(sourceRouter chi.Router) {
				sourceRouter.Use(a.M.RequireGroup())
				sourceRouter.Use(a.M.RequirePermission(auth.RoleAdmin))
				sourceRouter.Use(a.M.RequireBaseUrl())

				sourceRouter.Post("/", a.CreateSource)
				sourceRouter.Get("/{sourceID}", a.GetSourceByID)
				sourceRouter.With(a.M.Pagination).Get("/", a.LoadSourcesPaged)
				sourceRouter.Put("/{sourceID}", a.UpdateSource)
				sourceRouter.Delete("/{sourceID}", a.DeleteSource)
			})
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
			})
		})

		uiRouter.Post("/users/forgot-password", a.ForgotPassword)
		uiRouter.Post("/users/reset-password", a.ResetPassword)

		uiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.Post("/login", a.LoginUser)
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

				orgSubRouter.Route("/security", func(securityRouter chi.Router) {
					securityRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

					securityRouter.Post("/keys", a.CreateAPIKey)
					securityRouter.With(a.M.Pagination).Get("/keys", a.GetAPIKeys)
					securityRouter.Get("/keys/{keyID}", a.GetAPIKeyByID)
					securityRouter.Put("/keys/{keyID}", a.UpdateAPIKey)
					securityRouter.Put("/keys/{keyID}/revoke", a.RevokeAPIKey)
				})

				orgSubRouter.Route("/groups", func(groupRouter chi.Router) {
					groupRouter.Route("/", func(orgSubRouter chi.Router) {
						groupRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", a.CreateGroup)
						groupRouter.Get("/", a.GetGroups)
					})

					groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
						groupSubRouter.Use(a.M.RequireGroup())
						groupSubRouter.Use(a.M.RateLimitByGroupID())
						groupSubRouter.Use(a.M.RequireOrganisationGroupMember())

						groupSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Get("/", a.GetGroup)
						groupSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", a.UpdateGroup)
						groupSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", a.DeleteGroup)

						groupSubRouter.Route("/apps", func(appRouter chi.Router) {
							appRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

							appRouter.Route("/", func(appSubRouter chi.Router) {
								appSubRouter.Post("/", a.CreateApp)
								appRouter.With(a.M.Pagination).Get("/", a.GetApps)
							})

							appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
								appSubRouter.Use(a.M.RequireApp())
								appSubRouter.Get("/", a.GetApp)
								appSubRouter.Put("/", a.UpdateApp)
								appSubRouter.Delete("/", a.DeleteApp)

								appSubRouter.Route("/keys", func(keySubRouter chi.Router) {
									keySubRouter.Use(a.M.RequireBaseUrl())
									keySubRouter.Post("/", a.CreateAppPortalAPIKey)
								})

								appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
									endpointAppSubRouter.Post("/", a.CreateAppEndpoint)
									endpointAppSubRouter.Get("/", a.GetAppEndpoints)

									endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
										e.Use(a.M.RequireAppEndpoint())

										e.Get("/", a.GetAppEndpoint)
										e.Put("/", a.UpdateAppEndpoint)
										e.Delete("/", a.DeleteAppEndpoint)
									})
								})

								appSubRouter.Route("/devices", func(deviceRouter chi.Router) {
									deviceRouter.With(a.M.Pagination).Get("/", a.FindDevicesByAppID)
								})
							})
						})

						groupSubRouter.Route("/events", func(eventRouter chi.Router) {
							eventRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleAdmin))

							eventRouter.Post("/", a.CreateAppEvent)
							eventRouter.With(a.M.Pagination).Get("/", a.GetEventsPaged)

							eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
								eventSubRouter.Use(a.M.RequireEvent())
								eventSubRouter.Get("/", a.GetAppEvent)
								eventSubRouter.Put("/replay", a.ReplayAppEvent)
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
	})

	//App Portal API.
	router.Route("/portal", func(portalRouter chi.Router) {
		portalRouter.Use(a.M.JsonResponse)
		portalRouter.Use(a.M.SetupCORS)
		portalRouter.Use(a.M.RequireAuth())
		portalRouter.Use(a.M.RequireGroup())
		portalRouter.Use(a.M.RequireAppID())

		portalRouter.Route("/apps", func(appRouter chi.Router) {
			appRouter.Use(a.M.RequireAppPortalApplication())
			appRouter.Use(a.M.RequireAppPortalPermission(auth.RoleAdmin))

			appRouter.Get("/", a.GetApp)

			appRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
				endpointAppSubRouter.Get("/", a.GetAppEndpoints)
				endpointAppSubRouter.Post("/", a.CreateAppEndpoint)

				endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
					e.Use(a.M.RequireAppEndpoint())

					e.Get("/", a.GetAppEndpoint)
					e.Put("/", a.UpdateAppEndpoint)
				})
			})
		})

		portalRouter.Route("/events", func(eventRouter chi.Router) {
			eventRouter.Use(a.M.RequireAppPortalApplication())
			eventRouter.Use(a.M.RequireAppPortalPermission(auth.RoleAdmin))

			eventRouter.With(a.M.Pagination).Get("/", a.GetEventsPaged)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Use(a.M.RequireEvent())
				eventSubRouter.Get("/", a.GetAppEvent)
				eventSubRouter.Put("/replay", a.ReplayAppEvent)
			})
		})

		portalRouter.Route("/subscriptions", func(subsriptionRouter chi.Router) {
			subsriptionRouter.Use(a.M.RequireAppPortalApplication())
			subsriptionRouter.Use(a.M.RequireAppPortalPermission(auth.RoleAdmin))

			subsriptionRouter.Post("/", a.CreateSubscription)
			subsriptionRouter.With(a.M.Pagination).Get("/", a.GetSubscriptions)
			subsriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
			subsriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
			subsriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
		})

		portalRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
			eventDeliveryRouter.Use(a.M.RequireAppPortalApplication())
			eventDeliveryRouter.Use(a.M.RequireAppPortalPermission(auth.RoleAdmin))

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
	})

	router.Handle("/queue/monitoring/*", a.S.Queue.(*redisqueue.RedisQueue).Monitor())
	router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))
	router.HandleFunc("/*", reactRootHandler)

	metrics.RegisterQueueMetrics(a.S.Queue)
	metrics.RegisterDBMetrics(a.R.EventDeliveryRepo)
	prometheus.MustRegister(metrics.RequestDuration())

	return router
}
