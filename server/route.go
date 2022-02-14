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

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
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

		// rate limit all requests.
		v1Router.Use(httprate.LimitAll(convoy.RATE_LIMIT, convoy.RATE_LIMIT_DURATION))

		v1Router.Route("/v1", func(r chi.Router) {
			r.Use(middleware.AllowContentType("application/json"))
			r.Use(jsonResponse)
			r.Use(requireAuth())

			r.Route("/groups", func(groupRouter chi.Router) {
				groupRouter.Get("/", app.GetGroups)
				groupRouter.With(requirePermission(auth.RoleSuperUser)).Post("/", app.CreateGroup)

				groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
					groupSubRouter.Use(requireGroup(app.groupRepo))

					groupSubRouter.With(requirePermission(auth.RoleAdmin)).Get("/", app.GetGroup)
					groupSubRouter.With(requirePermission(auth.RoleSuperUser)).Put("/", app.UpdateGroup)
					groupSubRouter.With(requirePermission(auth.RoleSuperUser)).Delete("/", app.DeleteGroup)
				})
			})

			r.Route("/applications", func(appRouter chi.Router) {
				appRouter.Use(requireGroup(app.groupRepo))
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
				eventRouter.Use(requirePermission(auth.RoleAdmin))

				eventRouter.With(instrumentPath("/events")).Post("/", app.CreateAppEvent)
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
				securityRouter.Use(requirePermission(auth.RoleSuperUser))

				securityRouter.Post("/keys", app.CreateAPIKey)
				securityRouter.With(pagination).Get("/keys", app.GetAPIKeys)
				securityRouter.Get("/keys/{keyID}", app.GetAPIKeyByID)
				securityRouter.Put("/keys/{keyID}/revoke", app.RevokeAPIKey)
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
			appRouter.Use(requirePermission(auth.RoleUIAdmin))

			appRouter.Route("/", func(appSubRouter chi.Router) {
				appRouter.With(pagination).Get("/", app.GetApps)
			})

			appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
				appSubRouter.Use(requireApp(app.appRepo))
				appSubRouter.Get("/", app.GetApp)

				appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
					endpointAppSubRouter.Get("/", app.GetAppEndpoints)

					endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
						e.Use(requireAppEndpoint())

						e.Get("/", app.GetAppEndpoint)
					})
				})
			})
		})

		uiRouter.Route("/events", func(eventRouter chi.Router) {
			eventRouter.Use(requireGroup(app.groupRepo))
			eventRouter.Use(requirePermission(auth.RoleUIAdmin))

			eventRouter.With(pagination).Get("/", app.GetEventsPaged)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Use(requireEvent(app.eventRepo))
				eventSubRouter.Get("/", app.GetAppEvent)
			})
		})

		uiRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
			eventDeliveryRouter.Use(requireGroup(app.groupRepo))
			eventDeliveryRouter.Use(requirePermission(auth.RoleUIAdmin))

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
	router.HandleFunc("/*", reactRootHandler)

	return router
}

func New(cfg config.Configuration,
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	appRepo datastore.ApplicationRepository,
	apiKeyRepo datastore.APIKeyRepository,
	orgRepo datastore.GroupRepository,
	eventQueue queue.Queuer, logger logger.Logger, tracer tracer.Tracer, cache cache.Cache) *http.Server {

	app := newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, orgRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

	srv := &http.Server{
		Handler:      buildRoutes(app),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
	}

	prometheus.MustRegister(requestDuration)
	return srv
}
