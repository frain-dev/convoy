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

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
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

	// Public API.
	router.Route("/api", func(v1Router chi.Router) {

		// rate limit all requests.
		v1Router.Use(httprate.LimitAll(convoy.RATE_LIMIT, convoy.RATE_LIMIT_DURATION))

		v1Router.Route("/v1", func(r chi.Router) {
			r.Use(middleware.AllowContentType("application/json"))
			r.Use(jsonResponse)
			r.Use(requireAuth())
			r.Use(requireGroup(app.groupRepo))

			r.Route("/groups", func(groupRouter chi.Router) {
				groupRouter.Use(requirePermission(auth.RoleAdmin))

				groupRouter.Get("/", app.GetGroups)
				groupRouter.Post("/", app.CreateGroup)

				groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
					groupSubRouter.Use(requireGroup(app.groupRepo))

					groupSubRouter.Get("/", app.GetGroup)
					groupSubRouter.Put("/", app.UpdateGroup)
				})
			})

			r.Route("/applications", func(appRouter chi.Router) {
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
				eventRouter.With(instrumentPath("/events")).Post("/", app.CreateAppEvent)
				eventRouter.With(pagination).Get("/", app.GetEventsPaged)

				eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
					eventSubRouter.Use(requireEvent(app.eventRepo))

					eventSubRouter.Get("/", app.GetAppEvent)

					eventSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
						eventDeliveryRouter.Get("/", app.GetEventDeliveries)

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
			})
		})
	})

	// UI API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(jsonResponse)
		uiRouter.Use(requireAuth())
		uiRouter.Use(requirePermission(auth.RoleUIAdmin))

		uiRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
			dashboardRouter.With(fetchDashboardSummary(app.appRepo, app.eventRepo)).Get("/summary", app.GetDashboardSummary)
			dashboardRouter.With(fetchAllConfigDetails()).Get("/config", app.GetAllConfigDetails)
		})

		uiRouter.Route("/groups", func(groupRouter chi.Router) {

			groupRouter.Route("/", func(orgSubRouter chi.Router) {
				groupRouter.Get("/", app.GetGroups)
			})

			groupRouter.Route("/{groupID}", func(appSubRouter chi.Router) {
				appSubRouter.Get("/", app.GetGroup)
			})
		})

		uiRouter.Route("/apps", func(appRouter chi.Router) {

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
			eventRouter.With(pagination).Get("/", app.GetEventsPaged)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Use(requireEvent(app.eventRepo))

				eventSubRouter.Get("/", app.GetAppEvent)

				eventSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
					eventDeliveryRouter.With(pagination).Get("/", app.GetEventDeliveries)

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
		})
	})

	router.Handle("/v1/metrics", promhttp.Handler())
	router.HandleFunc("/*", reactRootHandler)

	return router
}

func New(cfg config.Configuration,
	eventRepo convoy.EventRepository,
	eventDeliveryRepo convoy.EventDeliveryRepository,
	appRepo convoy.ApplicationRepository,
	orgRepo convoy.GroupRepository,
	eventQueue queue.Queuer) *http.Server {

	app := newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, orgRepo, eventQueue)

	srv := &http.Server{
		Handler:      buildRoutes(app),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
	}

	prometheus.MustRegister(requestDuration)
	return srv
}
