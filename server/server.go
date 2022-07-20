package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/logger"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/searcher"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/tracer"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
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

type Server struct {
	s                         *http.Server
	m                         *middleware.Middleware
	cache                     cache.Cache
	queue                     queue.Queuer
	appService                *services.AppService
	eventService              *services.EventService
	groupService              *services.GroupService
	securityService           *services.SecurityService
	sourceService             *services.SourceService
	configService             *services.ConfigService
	userService               *services.UserService
	subService                *services.SubcriptionService
	organisationService       *services.OrganisationService
	organisationMemberService *services.OrganisationMemberService
	organisationInviteService *services.OrganisationInviteService

	// for crc check only
	sourceRepo datastore.SourceRepository
}

func NewServer(
	cfg config.Configuration,
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	appRepo datastore.ApplicationRepository,
	groupRepo datastore.GroupRepository,
	apiKeyRepo datastore.APIKeyRepository,
	subRepo datastore.SubscriptionRepository,
	sourceRepo datastore.SourceRepository,
	orgRepo datastore.OrganisationRepository,
	orgMemberRepo datastore.OrganisationMemberRepository,
	orgInviteRepo datastore.OrganisationInviteRepository,
	userRepo datastore.UserRepository,
	configRepo datastore.ConfigurationRepository,
	queue queue.Queuer,
	logger logger.Logger,
	tracer tracer.Tracer,
	cache cache.Cache,
	limiter limiter.RateLimiter, searcher searcher.Searcher) *Server {

	as := services.NewAppService(appRepo, eventRepo, eventDeliveryRepo, cache)
	es := services.NewEventService(appRepo, eventRepo, eventDeliveryRepo, queue, cache, searcher, subRepo)
	gs := services.NewGroupService(apiKeyRepo, appRepo, groupRepo, eventRepo, eventDeliveryRepo, limiter, cache)
	ss := services.NewSecurityService(groupRepo, apiKeyRepo)
	os := services.NewOrganisationService(orgRepo, orgMemberRepo)
	rs := services.NewSubscriptionService(subRepo, appRepo, sourceRepo)
	sos := services.NewSourceService(sourceRepo, cache)
	us := services.NewUserService(userRepo, cache, queue)
	ois := services.NewOrganisationInviteService(orgRepo, userRepo, orgMemberRepo, orgInviteRepo, queue)
	om := services.NewOrganisationMemberService(orgMemberRepo)
	cs := services.NewConfigService(configRepo)

	m := middleware.NewMiddleware(
		eventRepo, eventDeliveryRepo, appRepo,
		groupRepo, apiKeyRepo, subRepo, sourceRepo,
		orgRepo, orgMemberRepo, orgInviteRepo,
		userRepo, configRepo, cache, logger, limiter, tracer,
	)

	srv := &Server{
		s: &http.Server{
			ReadTimeout:  time.Second * 30,
			WriteTimeout: time.Second * 30,
			Addr:         fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
		},
		m:                         m,
		queue:                     queue,
		cache:                     cache,
		appService:                as,
		eventService:              es,
		groupService:              gs,
		securityService:           ss,
		sourceService:             sos,
		configService:             cs,
		userService:               us,
		subService:                rs,
		organisationService:       os,
		organisationMemberService: om,
		organisationInviteService: ois,

		sourceRepo: sourceRepo,
	}

	srv.s.Handler = srv.SetupRoutes()

	metrics.RegisterQueueMetrics(queue, cfg)
	metrics.RegisterDBMetrics(eventDeliveryRepo)
	prometheus.MustRegister(metrics.RequestDuration())
	return srv
}

func (s *Server) SetupRoutes() http.Handler {

	router := chi.NewRouter()

	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.Recoverer)
	router.Use(s.m.WriteRequestIDHeader)
	router.Use(s.m.InstrumentRequests())
	router.Use(s.m.LogHttpRequest())

	// Ingestion API
	router.Route("/ingest", func(ingestRouter chi.Router) {
		ingestRouter.Get("/{maskID}", s.HandleCrcCheck)
		ingestRouter.Post("/{maskID}", s.IngestEvent)
	})

	// Public API.
	router.Route("/api", func(v1Router chi.Router) {

		v1Router.Route("/v1", func(r chi.Router) {
			r.Use(chiMiddleware.AllowContentType("application/json"))
			r.Use(s.m.JsonResponse)
			r.Use(s.m.RequireAuth())

			r.Route("/applications", func(appRouter chi.Router) {
				appRouter.Use(s.m.RequireGroup())
				appRouter.Use(s.m.RateLimitByGroupID())
				appRouter.Use(s.m.RequirePermission(auth.RoleAdmin))

				appRouter.Route("/", func(appSubRouter chi.Router) {
					appSubRouter.Post("/", s.CreateApp)
					appRouter.With(s.m.Pagination).Get("/", s.GetApps)
				})

				appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
					appSubRouter.Use(s.m.RequireApp())

					appSubRouter.Get("/", s.GetApp)
					appSubRouter.Put("/", s.UpdateApp)
					appSubRouter.Delete("/", s.DeleteApp)

					appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
						endpointAppSubRouter.Post("/", s.CreateAppEndpoint)
						endpointAppSubRouter.Get("/", s.GetAppEndpoints)

						endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
							e.Use(s.m.RequireAppEndpoint())

							e.Get("/", s.GetAppEndpoint)
							e.Put("/", s.UpdateAppEndpoint)
							e.Delete("/", s.DeleteAppEndpoint)
						})
					})
				})
			})

			r.Route("/events", func(eventRouter chi.Router) {
				eventRouter.Use(s.m.RequireGroup())
				eventRouter.Use(s.m.RateLimitByGroupID())
				eventRouter.Use(s.m.RequirePermission(auth.RoleAdmin))

				eventRouter.With(s.m.InstrumentPath("/events")).Post("/", s.CreateAppEvent)
				eventRouter.With(s.m.Pagination).Get("/", s.GetEventsPaged)

				eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
					eventSubRouter.Use(s.m.RequireEvent())
					eventSubRouter.Get("/", s.GetAppEvent)
					eventSubRouter.Put("/replay", s.ReplayAppEvent)
				})
			})

			r.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
				eventDeliveryRouter.Use(s.m.RequireGroup())
				eventDeliveryRouter.Use(s.m.RequirePermission(auth.RoleAdmin))

				eventDeliveryRouter.With(s.m.Pagination).Get("/", s.GetEventDeliveriesPaged)
				eventDeliveryRouter.Post("/forceresend", s.ForceResendEventDeliveries)
				eventDeliveryRouter.Post("/batchretry", s.BatchRetryEventDelivery)
				eventDeliveryRouter.Get("/countbatchretryevents", s.CountAffectedEventDeliveries)

				eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
					eventDeliverySubRouter.Use(s.m.RequireEventDelivery())

					eventDeliverySubRouter.Get("/", s.GetEventDelivery)
					eventDeliverySubRouter.Put("/resend", s.ResendEventDelivery)

					eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
						deliveryRouter.Use(fetchDeliveryAttempts())

						deliveryRouter.Get("/", s.GetDeliveryAttempts)
						deliveryRouter.With(s.m.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", s.GetDeliveryAttempt)
					})
				})
			})

			r.Route("/security", func(securityRouter chi.Router) {
				securityRouter.Route("/applications/{appID}/keys", func(securitySubRouter chi.Router) {
					securitySubRouter.Use(s.m.RequireGroup())
					securitySubRouter.Use(s.m.RequirePermission(auth.RoleAdmin))
					securitySubRouter.Use(s.m.RequireApp())
					securitySubRouter.Use(s.m.RequireBaseUrl())
					securitySubRouter.Post("/", s.CreateAppPortalAPIKey)
				})
			})

			r.Route("/subscriptions", func(subscriptionRouter chi.Router) {
				subscriptionRouter.Use(s.m.RequireGroup())
				subscriptionRouter.Use(s.m.RateLimitByGroupID())
				subscriptionRouter.Use(s.m.RequirePermission(auth.RoleAdmin))

				subscriptionRouter.Post("/", s.CreateSubscription)
				subscriptionRouter.With(s.m.Pagination).Get("/", s.GetSubscriptions)
				subscriptionRouter.Delete("/{subscriptionID}", s.DeleteSubscription)
				subscriptionRouter.Get("/{subscriptionID}", s.GetSubscription)
				subscriptionRouter.Put("/{subscriptionID}", s.UpdateSubscription)
				subscriptionRouter.Put("/{subscriptionID}/toggle_status", s.ToggleSubscriptionStatus)
			})

			r.Route("/sources", func(sourceRouter chi.Router) {
				sourceRouter.Use(s.m.RequireGroup())
				sourceRouter.Use(s.m.RequirePermission(auth.RoleAdmin))
				sourceRouter.Use(s.m.RequireBaseUrl())

				sourceRouter.Post("/", s.CreateSource)
				sourceRouter.Get("/{sourceID}", s.GetSourceByID)
				sourceRouter.With(s.m.Pagination).Get("/", s.LoadSourcesPaged)
				sourceRouter.Put("/{sourceID}", s.UpdateSource)
				sourceRouter.Delete("/{sourceID}", s.DeleteSource)
			})
		})
	})

	// UI API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(s.m.JsonResponse)
		uiRouter.Use(s.m.SetupCORS)
		uiRouter.Use(chiMiddleware.Maybe(s.m.RequireAuth(), m.ShouldAuthRoute))
		uiRouter.Use(s.m.RequireBaseUrl())

		uiRouter.Post("/organisations/process_invite", s.ProcessOrganisationMemberInvite)
		uiRouter.Get("/users/token", s.FindUserByInviteToken)

		uiRouter.Route("/users", func(userRouter chi.Router) {
			userRouter.Use(s.m.RequireAuthUserMetadata())
			userRouter.Route("/{userID}", func(userSubRouter chi.Router) {
				userSubRouter.Use(s.m.RequireAuthorizedUser())
				userSubRouter.Get("/profile", s.GetUser)
				userSubRouter.Put("/profile", s.UpdateUser)
				userSubRouter.Put("/password", s.UpdatePassword)
			})
		})

		uiRouter.Post("/users/forgot-password", s.ForgotPassword)
		uiRouter.Post("/users/reset-password", s.ResetPassword)

		uiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.Post("/login", s.LoginUser)
			authRouter.Post("/token/refresh", s.RefreshToken)
			authRouter.Post("/logout", s.LogoutUser)
		})

		uiRouter.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Use(s.m.RequireAuthUserMetadata())
			orgRouter.Use(s.m.RequireBaseUrl())

			orgRouter.Post("/", s.CreateOrganisation)
			orgRouter.With(s.m.Pagination).Get("/", s.GetOrganisationsPaged)

			orgRouter.Route("/{orgID}", func(orgSubRouter chi.Router) {
				orgSubRouter.Use(s.m.RequireOrganisation())
				orgSubRouter.Use(s.m.RequireOrganisationMembership())

				orgSubRouter.Get("/", s.GetOrganisation)
				orgSubRouter.With(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", s.UpdateOrganisation)
				orgSubRouter.With(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", s.DeleteOrganisation)

				orgSubRouter.Route("/invites", func(orgInvitesRouter chi.Router) {
					orgInvitesRouter.With(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", s.InviteUserToOrganisation)
					orgInvitesRouter.With(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/resend", s.ResendOrganizationInvite)
					orgInvitesRouter.With(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/cancel", s.CancelOrganizationInvite)
					orgInvitesRouter.With(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser)).With(s.m.Pagination).Get("/pending", s.GetPendingOrganisationInvites)
				})

				orgSubRouter.Route("/members", func(orgMemberRouter chi.Router) {
					orgMemberRouter.Use(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser))

					orgMemberRouter.With(s.m.Pagination).Get("/", s.GetOrganisationMembers)

					orgMemberRouter.Route("/{memberID}", func(orgMemberSubRouter chi.Router) {

						orgMemberSubRouter.Get("/", s.GetOrganisationMember)
						orgMemberSubRouter.Put("/", s.UpdateOrganisationMember)
						orgMemberSubRouter.Delete("/", s.DeleteOrganisationMember)

					})
				})

				orgSubRouter.Route("/security", func(securityRouter chi.Router) {
					securityRouter.Use(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser))

					securityRouter.Post("/keys", s.CreateAPIKey)
					securityRouter.With(s.m.Pagination).Get("/keys", s.GetAPIKeys)
					securityRouter.Get("/keys/{keyID}", s.GetAPIKeyByID)
					securityRouter.Put("/keys/{keyID}", s.UpdateAPIKey)
					securityRouter.Put("/keys/{keyID}/revoke", s.RevokeAPIKey)
				})

				orgSubRouter.Route("/groups", func(groupRouter chi.Router) {
					groupRouter.Route("/", func(orgSubRouter chi.Router) {
						groupRouter.With(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", s.CreateGroup)
						groupRouter.Get("/", s.GetGroups)
					})

					groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
						groupSubRouter.Use(s.m.RequireGroup())
						groupSubRouter.Use(s.m.RateLimitByGroupID())
						groupSubRouter.Use(s.m.RequireOrganisationGroupMember())

						groupSubRouter.With(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Get("/", s.GetGroup)
						groupSubRouter.With(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", s.UpdateGroup)
						groupSubRouter.With(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", s.DeleteGroup)

						groupSubRouter.Route("/apps", func(appRouter chi.Router) {
							appRouter.Use(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser))

							appRouter.Route("/", func(appSubRouter chi.Router) {
								appSubRouter.Post("/", s.CreateApp)
								appRouter.With(s.m.Pagination).Get("/", s.GetApps)
							})

							appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
								appSubRouter.Use(s.m.RequireApp())
								appSubRouter.Get("/", s.GetApp)
								appSubRouter.Put("/", s.UpdateApp)
								appSubRouter.Delete("/", s.DeleteApp)

								appSubRouter.Route("/keys", func(keySubRouter chi.Router) {
									keySubRouter.Use(s.m.RequireBaseUrl())
									keySubRouter.Post("/", s.CreateAppPortalAPIKey)
								})

								appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
									endpointAppSubRouter.Post("/", s.CreateAppEndpoint)
									endpointAppSubRouter.Get("/", s.GetAppEndpoints)

									endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
										e.Use(s.m.RequireAppEndpoint())

										e.Get("/", s.GetAppEndpoint)
										e.Put("/", s.UpdateAppEndpoint)
										e.Delete("/", s.DeleteAppEndpoint)
									})
								})
							})
						})

						groupSubRouter.Route("/events", func(eventRouter chi.Router) {
							eventRouter.Use(s.m.RequireOrganisationMemberRole(auth.RoleAdmin))

							eventRouter.Post("/", s.CreateAppEvent)
							eventRouter.With(s.m.Pagination).Get("/", s.GetEventsPaged)

							eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
								eventSubRouter.Use(s.m.RequireEvent())
								eventSubRouter.Get("/", s.GetAppEvent)
								eventSubRouter.Put("/replay", s.ReplayAppEvent)
							})
						})

						groupSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
							eventDeliveryRouter.Use(s.m.RequireOrganisationMemberRole(auth.RoleSuperUser))

							eventDeliveryRouter.With(s.m.Pagination).Get("/", s.GetEventDeliveriesPaged)
							eventDeliveryRouter.Post("/forceresend", s.ForceResendEventDeliveries)
							eventDeliveryRouter.Post("/batchretry", s.BatchRetryEventDelivery)
							eventDeliveryRouter.Get("/countbatchretryevents", s.CountAffectedEventDeliveries)

							eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
								eventDeliverySubRouter.Use(s.m.RequireEventDelivery())

								eventDeliverySubRouter.Get("/", s.GetEventDelivery)
								eventDeliverySubRouter.Put("/resend", s.ResendEventDelivery)

								eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
									deliveryRouter.Use(fetchDeliveryAttempts())

									deliveryRouter.Get("/", s.GetDeliveryAttempts)
									deliveryRouter.With(s.m.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", s.GetDeliveryAttempt)
								})
							})
						})

						groupSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
							subscriptionRouter.Use(s.m.RequireOrganisationMemberRole(auth.RoleAdmin))

							subscriptionRouter.Post("/", s.CreateSubscription)
							subscriptionRouter.With(s.m.Pagination).Get("/", s.GetSubscriptions)
							subscriptionRouter.Delete("/{subscriptionID}", s.DeleteSubscription)
							subscriptionRouter.Get("/{subscriptionID}", s.GetSubscription)
							subscriptionRouter.Put("/{subscriptionID}", s.UpdateSubscription)
						})

						groupSubRouter.Route("/sources", func(sourceRouter chi.Router) {
							sourceRouter.Use(s.m.RequireOrganisationMemberRole(auth.RoleAdmin))
							sourceRouter.Use(s.m.RequireBaseUrl())

							sourceRouter.Post("/", s.CreateSource)
							sourceRouter.Get("/{sourceID}", s.GetSourceByID)
							sourceRouter.With(s.m.Pagination).Get("/", s.LoadSourcesPaged)
							sourceRouter.Put("/{sourceID}", s.UpdateSource)
							sourceRouter.Delete("/{sourceID}", s.DeleteSource)
						})

						groupSubRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
							dashboardRouter.Get("/summary", s.GetDashboardSummary)
							dashboardRouter.Get("/config", s.GetAllConfigDetails)
						})
					})

				})
			})
		})

		uiRouter.Route("/configuration", func(configRouter chi.Router) {
			configRouter.Use(s.m.RequireAuthUserMetadata())

			configRouter.Get("/", s.LoadConfiguration)
			configRouter.Post("/", s.CreateConfiguration)
			configRouter.Put("/", s.UpdateConfiguration)

		})
	})

	//App Portal API.
	router.Route("/portal", func(portalRouter chi.Router) {
		portalRouter.Use(s.m.JsonResponse)
		portalRouter.Use(s.m.SetupCORS)
		portalRouter.Use(s.m.RequireAuth())
		portalRouter.Use(s.m.RequireGroup())
		portalRouter.Use(s.m.RequireAppID())

		portalRouter.Route("/apps", func(appRouter chi.Router) {
			appRouter.Use(s.m.RequireAppPortalApplication())
			appRouter.Use(s.m.RequireAppPortalPermission(auth.RoleAdmin))

			appRouter.Get("/", s.GetApp)

			appRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
				endpointAppSubRouter.Get("/", s.GetAppEndpoints)
				endpointAppSubRouter.Post("/", s.CreateAppEndpoint)

				endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
					e.Use(s.m.RequireAppEndpoint())

					e.Get("/", s.GetAppEndpoint)
					e.Put("/", s.UpdateAppEndpoint)
				})
			})
		})

		portalRouter.Route("/events", func(eventRouter chi.Router) {
			eventRouter.Use(s.m.RequireAppPortalApplication())
			eventRouter.Use(s.m.RequireAppPortalPermission(auth.RoleAdmin))

			eventRouter.With(s.m.Pagination).Get("/", s.GetEventsPaged)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Use(s.m.RequireEvent())
				eventSubRouter.Get("/", s.GetAppEvent)
				eventSubRouter.Put("/replay", s.ReplayAppEvent)
			})
		})

		portalRouter.Route("/subscriptions", func(subsriptionRouter chi.Router) {
			subsriptionRouter.Use(s.m.RequireAppPortalApplication())
			subsriptionRouter.Use(s.m.RequireAppPortalPermission(auth.RoleAdmin))

			subsriptionRouter.Post("/", s.CreateSubscription)
			subsriptionRouter.With(s.m.Pagination).Get("/", s.GetSubscriptions)
			subsriptionRouter.Delete("/{subscriptionID}", s.DeleteSubscription)
			subsriptionRouter.Get("/{subscriptionID}", s.GetSubscription)
			subsriptionRouter.Put("/{subscriptionID}", s.UpdateSubscription)
		})

		portalRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
			eventDeliveryRouter.Use(s.m.RequireAppPortalApplication())
			eventDeliveryRouter.Use(s.m.RequireAppPortalPermission(auth.RoleAdmin))

			eventDeliveryRouter.With(s.m.Pagination).Get("/", s.GetEventDeliveriesPaged)
			eventDeliveryRouter.Post("/forceresend", s.ForceResendEventDeliveries)
			eventDeliveryRouter.Post("/batchretry", s.BatchRetryEventDelivery)
			eventDeliveryRouter.Get("/countbatchretryevents", s.CountAffectedEventDeliveries)

			eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
				eventDeliverySubRouter.Use(s.m.RequireEventDelivery())

				eventDeliverySubRouter.Get("/", s.GetEventDelivery)
				eventDeliverySubRouter.Put("/resend", s.ResendEventDelivery)

				eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
					deliveryRouter.Use(fetchDeliveryAttempts())

					deliveryRouter.Get("/", s.GetDeliveryAttempts)
					deliveryRouter.With(s.m.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", s.GetDeliveryAttempt)
				})
			})
		})
	})

	router.Handle("/queue/monitoring/*", s.queue.(*redisqueue.RedisQueue).Monitor())
	router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("Convoy %v", convoy.GetVersion()), nil, http.StatusOK))
	})
	router.HandleFunc("/*", reactRootHandler)

	return router
}

func (s *Server) Listen() {

	go func() {
		//service connections
		if err := s.s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("failed to listen")
		}
	}()

	s.gracefulShutdown()
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) {
	go func() {
		//service connections
		if err := s.s.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("failed to listen")
		}
	}()

	s.gracefulShutdown()
}

func (s *Server) gracefulShutdown() {
	//Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Info("Stopping websocket server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.s.Shutdown(ctx); err != nil {
		log.WithError(err).Fatal("Server Shutdown")
	}

	log.Info("Websocket server exiting")

	time.Sleep(2 * time.Second) // allow all websocket connections close themselves
}
