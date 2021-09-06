package server

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/go-chi/chi/v5"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/config"
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

	router.Route("/v1", func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Use(jsonResponse)

		r.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Use(requireAuth())

			orgRouter.Route("/", func(orgSubRouter chi.Router) {
				orgSubRouter.With(ensureNewOrganisation(app.orgRepo)).Post("/", app.CreateOrganisation)

				orgRouter.With(fetchAllOrganisations(app.orgRepo)).Get("/", app.GetOrganisations)
			})

			orgRouter.Route("/{orgID}", func(appSubRouter chi.Router) {
				appSubRouter.Use(requireOrganisation(app.orgRepo))

				appSubRouter.With(ensureOrganisationUpdate(app.orgRepo)).Put("/", app.UpdateOrganisation)

				appSubRouter.Get("/", app.GetOrganisation)
			})
		})

		r.Route("/apps", func(appRouter chi.Router) {
			appRouter.Use(requireAuth())

			appRouter.Route("/", func(appSubRouter chi.Router) {
				appSubRouter.With(ensureNewApp(app.orgRepo, app.appRepo)).Post("/", app.CreateApp)

				appRouter.With(fetchAllApps(app.appRepo)).Get("/", app.GetApps)
			})

			appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
				appSubRouter.Use(requireApp(app.appRepo))

				appSubRouter.With(ensureAppUpdate(app.appRepo)).Put("/", app.UpdateApp)

				appSubRouter.Get("/", app.GetApp)
				appSubRouter.With(ensureAppDeletion(app.appRepo)).Delete("/", app.DeleteApp)

				appSubRouter.Route("/events", func(msgSubRouter chi.Router) {
					msgSubRouter.With(instrumentPath("/events"), ensureNewMessage(app.appRepo, app.msgRepo)).Post("/", app.CreateAppMessage)
					msgSubRouter.With(pagination).With(fetchAppMessages(app.msgRepo)).Get("/", app.GetAppMessagesPaged)

					msgSubRouter.Route("/{eventID}", func(msgEventSubRouter chi.Router) {
						msgEventSubRouter.Use(requireMessage(app.msgRepo))

						msgEventSubRouter.Get("/", app.GetAppMessage)
					})
				})

				appSubRouter.Route("/endpoint", func(endpointAppSubRouter chi.Router) {
					endpointAppSubRouter.With(ensureNewAppEndpoint(app.appRepo)).Post("/", app.CreateAppEndpoint)
					endpointAppSubRouter.With(fetchAppEndpoints()).Get("/", app.GetAppEndpoints)

					endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
						e.Use(requireAppEndpoint())

						e.Get("/", app.GetAppEndpoint)
						e.With(ensureAppEndpointUpdate(app.appRepo)).Put("/", app.UpdateAppEndpoint)
						e.With(ensureAppEndpointDeletion(app.appRepo)).Delete("/", app.DeleteAppEndpoint)
					})
				})
			})
		})

		r.Route("/events", func(msgRouter chi.Router) {
			msgRouter.Use(requireAuth())

			msgRouter.With(pagination).With(fetchAllMessages(app.msgRepo)).Get("/", app.GetAppMessagesPaged)

			msgRouter.Route("/{eventID}", func(msgSubRouter chi.Router) {
				msgSubRouter.Use(requireMessage(app.msgRepo))

				msgSubRouter.Get("/", app.GetAppMessage)

				msgSubRouter.With(resendMessage(app.msgRepo)).Put("/resend", app.ResendAppMessage)

				msgSubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
					deliveryRouter.Use(fetchMessageDeliveryAttempts())

					deliveryRouter.Get("/", app.GetAppMessageDeliveryAttempts)

					deliveryRouter.With(requireMessageDeliveryAttempt()).Get("/{deliveryAttemptID}", app.GetAppMessageDeliveryAttempt)
				})
			})

		})

		r.Route("/dashboard/{orgID}", func(dashboardRouter chi.Router) {
			dashboardRouter.Use(requireAuth())

			dashboardRouter.Use(requireOrganisation(app.orgRepo))

			dashboardRouter.With(fetchDashboardSummary(app.appRepo, app.msgRepo)).Get("/summary", app.GetDashboardSummary)
			dashboardRouter.With(pagination).With(fetchOrganisationApps(app.appRepo)).Get("/apps", app.GetPaginatedApps)
		})

		r.Route("/auth", func(authRouter chi.Router) {
			authRouter.With(fetchAuthConfig()).Get("/details", app.GetAuthDetails)
		})
	})

	router.Handle("/metrics", promhttp.Handler())
	router.HandleFunc("/*", reactRootHandler)

	return router
}

func New(cfg config.Configuration, msgRepo hookcamp.MessageRepository, appRepo hookcamp.ApplicationRepository, orgRepo hookcamp.OrganisationRepository) *http.Server {

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
