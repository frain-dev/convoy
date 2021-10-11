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
	"github.com/frain-dev/convoy/config"

	"github.com/go-chi/chi/v5/middleware"
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
		log.Errorf("an error has occurred with the react app - %+v\n", err)
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
	router.Route("/v1", func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Use(jsonResponse)

		r.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Use(requireAuth())

			orgRouter.Route("/", func(orgSubRouter chi.Router) {
				orgRouter.Get("/", app.GetOrganisations)
				orgSubRouter.Post("/", app.CreateOrganisation)
			})

			orgRouter.Route("/{orgID}", func(appSubRouter chi.Router) {
				appSubRouter.Use(requireOrganisation(app.orgRepo))

				appSubRouter.Get("/", app.GetOrganisation)
				appSubRouter.Put("/", app.UpdateOrganisation)
			})
		})

		r.Route("/applications", func(appRouter chi.Router) {
			appRouter.Use(requireAuth())

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

		r.Route("/events", func(msgRouter chi.Router) {
			msgRouter.Use(requireAuth())

			msgRouter.With(instrumentPath("/events")).Post("/", app.CreateAppMessage)
			msgRouter.With(pagination).Get("/", app.GetMessagesPaged) //TODO(subomi,daniel): this should have /applications/{appID} and be renamed to GetAppMessagesPaged or do we pass the appID param in the request body

			msgRouter.Route("/{eventID}", func(msgSubRouter chi.Router) {
				msgSubRouter.Use(requireMessage(app.msgRepo))

				msgSubRouter.Get("/", app.GetAppMessage)
				msgSubRouter.Put("/resend", app.ResendAppMessage)

				msgSubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
					deliveryRouter.Use(fetchMessageDeliveryAttempts())

					deliveryRouter.Get("/", app.GetAppMessageDeliveryAttempts)
					deliveryRouter.With(requireMessageDeliveryAttempt()).Get("/{deliveryAttemptID}", app.GetAppMessageDeliveryAttempt)
				})
			})

		})
	})

	// UI API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(jsonResponse)

		uiRouter.Route("/dashboard/{orgID}", func(dashboardRouter chi.Router) {
			dashboardRouter.Use(requireUIAuth())

			dashboardRouter.Use(requireOrganisation(app.orgRepo))

			dashboardRouter.With(fetchDashboardSummary(app.appRepo, app.msgRepo)).Get("/summary", app.GetDashboardSummary)
			dashboardRouter.With(pagination).With(fetchOrganisationApps(app.appRepo)).Get("/apps", app.GetPaginatedApps)

			dashboardRouter.Route("/events/{eventID}", func(msgSubRouter chi.Router) {
				msgSubRouter.Use(requireMessage(app.msgRepo))

				msgSubRouter.Put("/resend", app.ResendAppMessage)
			})

			dashboardRouter.With(fetchAllConfigDetails()).Get("/config", app.GetAllConfigDetails)
		})

		uiRouter.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Use(requireUIAuth())

			orgRouter.Route("/", func(orgSubRouter chi.Router) {
				orgRouter.Get("/", app.GetOrganisations)
			})

			orgRouter.Route("/{orgID}", func(appSubRouter chi.Router) {
				appSubRouter.Use(requireOrganisation(app.orgRepo))
				appSubRouter.Get("/", app.GetOrganisation)
			})
		})

		uiRouter.Route("/apps", func(appRouter chi.Router) {
			appRouter.Use(requireUIAuth())

			appRouter.Route("/", func(appSubRouter chi.Router) {
				appRouter.With(pagination).Get("/", app.GetApps)
			})

			appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
				appSubRouter.Use(requireApp(app.appRepo))
				appSubRouter.Get("/", app.GetApp)
				appSubRouter.Route("/events", func(msgSubRouter chi.Router) {
					msgSubRouter.With(pagination).Get("/", app.GetMessagesPaged)

					msgSubRouter.Route("/{eventID}", func(msgEventSubRouter chi.Router) {
						msgEventSubRouter.Use(requireMessage(app.msgRepo))

						msgEventSubRouter.Get("/", app.GetAppMessage)
						msgEventSubRouter.Put("/resend", app.ResendAppMessage)
					})
				})

				appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
					endpointAppSubRouter.Get("/", app.GetAppEndpoints)

					endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
						e.Use(requireAppEndpoint())

						e.Get("/", app.GetAppEndpoint)
					})
				})
			})
		})

		uiRouter.Route("/events", func(msgRouter chi.Router) {
			msgRouter.Use(requireUIAuth())
			msgRouter.With(pagination).With(fetchAllMessages(app.msgRepo)).Get("/", app.GetMessagesPaged)

			msgRouter.Route("/{eventID}", func(msgSubRouter chi.Router) {
				msgSubRouter.Use(requireMessage(app.msgRepo))

				msgSubRouter.Get("/", app.GetAppMessage)

				msgSubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
					deliveryRouter.Use(fetchMessageDeliveryAttempts())

					deliveryRouter.Get("/", app.GetAppMessageDeliveryAttempts)

					deliveryRouter.With(requireMessageDeliveryAttempt()).Get("/{deliveryAttemptID}", app.GetAppMessageDeliveryAttempt)
				})
			})

		})

		uiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.With(login()).Post("/login", app.GetAuthLogin)
			authRouter.With(refresh()).Post("/refresh", app.GetAuthLogin)
		})

	})

	router.Handle("/v1/metrics", promhttp.Handler())
	router.HandleFunc("/*", reactRootHandler)

	return router
}

func New(cfg config.Configuration, msgRepo convoy.MessageRepository, appRepo convoy.ApplicationRepository, orgRepo convoy.OrganisationRepository) *http.Server {

	app := newApplicationHandler(msgRepo, appRepo, orgRepo)

	srv := &http.Server{
		Handler:      buildRoutes(app),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
	}

	prometheus.MustRegister(requestDuration)

	return srv
}
