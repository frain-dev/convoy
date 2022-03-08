package server

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/logger"
	"github.com/frain-dev/convoy/tracer"
	"github.com/frain-dev/convoy/worker"

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
	router.Use(writeRequestIDHeader)
	router.Use(instrumentRequests(app.tracer))
	router.Use(logHttpRequest(app.logger))

	// Public API.
	router.Route("/api", func(v1Router chi.Router) {

		v1Router.Route("/v1", func(r chi.Router) {
			r.Use(middleware.AllowContentType("application/json"))
			r.Use(jsonResponse)
			r.Use(requireAuth())

			r.Route("/groups", func(groupRouter chi.Router) {
				groupRouter.Get("/", app.GetGroups)
				groupRouter.With(requirePermission(auth.RoleSuperUser)).Post("/", app.CreateGroup)

				groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
					groupSubRouter.Use(requireGroup(app.groupRepo))
					groupSubRouter.Use(rateLimitByGroupID(app.limiter))

					groupSubRouter.With(requirePermission(auth.RoleAdmin)).Get("/", app.GetGroup)
					groupSubRouter.With(requirePermission(auth.RoleSuperUser)).Put("/", app.UpdateGroup)
					groupSubRouter.With(requirePermission(auth.RoleSuperUser)).Delete("/", app.DeleteGroup)
				})
			})

			r.Route("/applications", func(appRouter chi.Router) {
				appRouter.Use(requireGroup(app.groupRepo))
				appRouter.Use(rateLimitByGroupID(app.limiter))
				appRouter.Use(requirePermission(auth.RoleAdmin))

				appRouter.Route("/", func(appSubRouter chi.Router) {
					appSubRouter.Post("/", app.CreateApp)
					appRouter.With(pagination).Get("/", app.GetApps)
				})

				appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
					appSubRouter.Use(requireApp(app.appRepo))

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
				eventRouter.Use(requireGroup(app.groupRepo))
				eventRouter.Use(rateLimitByGroupID(app.limiter))
				eventRouter.Use(requirePermission(auth.RoleAdmin))

				eventRouter.With(rateLimitByGroup(), instrumentPath("/events")).Post("/", app.CreateAppEvent)
				eventRouter.With(pagination).Get("/", app.GetEventsPaged)

				eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
					eventSubRouter.Use(requireEvent(app.eventRepo))
					eventSubRouter.Get("/", app.GetAppEvent)
				})
			})

			r.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
				eventDeliveryRouter.Use(requireGroup(app.groupRepo))
				eventDeliveryRouter.Use(requirePermission(auth.RoleAdmin))

				eventDeliveryRouter.With(pagination).Get("/", app.GetEventDeliveriesPaged)
				eventDeliveryRouter.Post("/forceresend", app.ForceResendEventDeliveries)
				eventDeliveryRouter.Post("/batchretry", app.BatchRetryEventDelivery)

				eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
					eventDeliverySubRouter.Use(requireEventDelivery(app.eventDeliveryRepo))

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
				securityRouter.Route("/", func(securitySubRouter chi.Router) {
					securitySubRouter.Use(requirePermission(auth.RoleSuperUser))

					securitySubRouter.Post("/keys", app.CreateAPIKey)
					securitySubRouter.With(pagination).Get("/keys", app.GetAPIKeys)
					securitySubRouter.Get("/keys/{keyID}", app.GetAPIKeyByID)
					securitySubRouter.Put("/keys/{keyID}", app.UpdateAPIKey)
					securitySubRouter.Put("/keys/{keyID}/revoke", app.RevokeAPIKey)
				})

				securityRouter.Route("/applications/{appID}/keys", func(securitySubRouter chi.Router) {
					securitySubRouter.Use(requirePermission(auth.RoleAdmin))
					securitySubRouter.Use(requireGroup(app.groupRepo))
					securitySubRouter.Use(requireApp(app.appRepo))
					securitySubRouter.Use(requireBaseUrl())
					securitySubRouter.Post("/", app.CreateAppPortalAPIKey)
				})
			})
		})
	})

	// UI API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(jsonResponse)
		uiRouter.Use(setupCORS)
		uiRouter.Use(requireAuth())

		uiRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
			dashboardRouter.Use(requireGroup(app.groupRepo))
			dashboardRouter.Use(rateLimitByGroupID(app.limiter))

			dashboardRouter.Get("/summary", app.GetDashboardSummary)
			dashboardRouter.Get("/config", app.GetAllConfigDetails)
		})

		uiRouter.Route("/groups", func(groupRouter chi.Router) {

			groupRouter.Route("/", func(orgSubRouter chi.Router) {
				groupRouter.Get("/", app.GetGroups)
			})

			groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
				groupSubRouter.With(requirePermission(auth.RoleUIAdmin)).Get("/", app.GetGroup)
				groupSubRouter.With(requirePermission(auth.RoleSuperUser)).Delete("/", app.DeleteGroup)
			})
		})

		uiRouter.Route("/apps", func(appRouter chi.Router) {
			appRouter.Use(requireGroup(app.groupRepo))
			appRouter.Use(rateLimitByGroupID(app.limiter))
			appRouter.Use(requirePermission(auth.RoleUIAdmin))

			appRouter.Route("/", func(appSubRouter chi.Router) {
				appSubRouter.Post("/", app.CreateApp)
				appRouter.With(pagination).Get("/", app.GetApps)
			})

			appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
				appSubRouter.Use(requireApp(app.appRepo))
				appSubRouter.Get("/", app.GetApp)
				appSubRouter.Put("/", app.UpdateApp)
				appSubRouter.Delete("/", app.DeleteApp)

				appSubRouter.Route("/keys", func(keySubRouter chi.Router) {
					keySubRouter.Use(requireGroup(app.groupRepo))
					keySubRouter.Use(requireApp(app.appRepo))
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

		uiRouter.Route("/events", func(eventRouter chi.Router) {
			eventRouter.Use(requireGroup(app.groupRepo))
			eventRouter.Use(rateLimitByGroupID(app.limiter))
			eventRouter.Use(requirePermission(auth.RoleUIAdmin))

			eventRouter.Post("/", app.CreateAppEvent)
			eventRouter.With(pagination).Get("/", app.GetEventsPaged)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Use(requireEvent(app.eventRepo))
				eventSubRouter.Get("/", app.GetAppEvent)
			})
		})

		uiRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
			eventDeliveryRouter.Use(requireGroup(app.groupRepo))
			eventDeliveryRouter.Use(rateLimitByGroupID(app.limiter))
			eventDeliveryRouter.Use(requirePermission(auth.RoleUIAdmin))

			eventDeliveryRouter.With(pagination).Get("/", app.GetEventDeliveriesPaged)
			eventDeliveryRouter.Post("/forceresend", app.ForceResendEventDeliveries)
			eventDeliveryRouter.Post("/batchretry", app.BatchRetryEventDelivery)

			eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
				eventDeliverySubRouter.Use(requireEventDelivery(app.eventDeliveryRepo))

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

	//App Portal API.
	router.Route("/portal", func(portalRouter chi.Router) {
		portalRouter.Use(jsonResponse)
		portalRouter.Use(setupCORS)
		portalRouter.Use(requireAuth())
		portalRouter.Use(requireGroup(app.groupRepo))

		portalRouter.Route("/apps", func(appRouter chi.Router) {
			appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
				appSubRouter.Use(requireAppPortalApplication(app.appRepo))
				appSubRouter.Use(requireAppPortalPermission(auth.RoleUIAdmin))

				appSubRouter.Get("/", app.GetApp)

				appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
					endpointAppSubRouter.Get("/", app.GetAppEndpoints)
					endpointAppSubRouter.Post("/", app.CreateAppEndpoint)

					endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
						e.Use(requireAppEndpoint())

						e.Get("/", app.GetAppEndpoint)
						e.Put("/", app.UpdateAppEndpoint)
					})
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
			})
		})

		portalRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
			eventDeliveryRouter.Use(requireAppPortalApplication(app.appRepo))
			eventDeliveryRouter.Use(requireAppPortalPermission(auth.RoleUIAdmin))

			eventDeliveryRouter.With(pagination).Get("/", app.GetEventDeliveriesPaged)
			eventDeliveryRouter.Post("/batchretry", app.BatchRetryEventDelivery)

			eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
				eventDeliverySubRouter.Use(requireEventDelivery(app.eventDeliveryRepo))

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

	router.Handle("/v1/metrics", promhttp.Handler())
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = render.Render(w, r, newServerResponse("Convoy", nil, http.StatusOK))
	})
	router.HandleFunc("/*", reactRootHandler)

	return router
}

func New(cfg config.Configuration,
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	appRepo datastore.ApplicationRepository,
	apiKeyRepo datastore.APIKeyRepository,
	orgRepo datastore.GroupRepository,
	eventQueue queue.Queuer,
	logger logger.Logger,
	tracer tracer.Tracer,
	cache cache.Cache,
	limiter limiter.RateLimiter) *http.Server {

	app := newApplicationHandler(
		eventRepo,
		eventDeliveryRepo,
		appRepo,
		orgRepo,
		apiKeyRepo,
		eventQueue,
		logger,
		tracer,
		cache,
		limiter)

	srv := &http.Server{
		Handler:      buildRoutes(app),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
	}

	RegisterQueueMetrics(eventQueue, cfg)
	worker.RegisterWorkerMetrics(eventQueue, cfg)
	prometheus.MustRegister(requestDuration)
	return srv
}
