package server

import (
	"crypto/tls"
	"crypto/x509"
	"embed"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"time"

	"go.step.sm/crypto/pemutil"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"

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
		v1Router.Route("/v1", func(r chi.Router) {
			r.Use(middleware.AllowContentType("application/json"))
			r.Use(jsonResponse)

			r.Route("/groups", func(groupRouter chi.Router) {
				groupRouter.Use(requireAuth())

				groupRouter.Get("/", app.GetGroups)
				groupRouter.Post("/", app.CreateGroup)

				groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
					groupSubRouter.Use(requireDefaultGroup(app.groupRepo))

					groupSubRouter.Get("/", app.GetGroup)
					groupSubRouter.Put("/", app.UpdateGroup)
				})
			})

			r.Route("/applications", func(appRouter chi.Router) {
				appRouter.Use(requireAuth())

				appRouter.Route("/", func(appSubRouter chi.Router) {
					appSubRouter.With(requireDefaultGroup(app.groupRepo)).Post("/", app.CreateApp)
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
				msgRouter.With(pagination).Get("/", app.GetMessagesPaged)

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
	})

	// UI API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(jsonResponse)

		uiRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
			dashboardRouter.Use(requireUIAuth())

			dashboardRouter.Use(requireDefaultGroup(app.groupRepo))

			dashboardRouter.With(fetchDashboardSummary(app.appRepo, app.msgRepo)).Get("/summary", app.GetDashboardSummary)
			dashboardRouter.With(pagination).With(fetchGroupApps(app.appRepo)).Get("/apps", app.GetPaginatedApps)

			dashboardRouter.Route("/events/{eventID}", func(msgSubRouter chi.Router) {
				msgSubRouter.Use(requireMessage(app.msgRepo))

				msgSubRouter.Put("/resend", app.ResendAppMessage)
			})

			dashboardRouter.With(fetchAllConfigDetails()).Get("/config", app.GetAllConfigDetails)
		})

		// TODO(daniel,subomi): maybe we should remove this? since we're now giving only a default group
		uiRouter.Route("/groups", func(groupRouter chi.Router) {
			groupRouter.Use(requireUIAuth())

			groupRouter.Route("/", func(orgSubRouter chi.Router) {
				groupRouter.Get("/", app.GetGroups)
			})

			groupRouter.Route("/{groupID}", func(appSubRouter chi.Router) {
				appSubRouter.Use(requireDefaultGroup(app.groupRepo))
				appSubRouter.Get("/", app.GetGroup)
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

func New(cfg config.Configuration, msgRepo convoy.MessageRepository, appRepo convoy.ApplicationRepository, orgRepo convoy.GroupRepository, scheduleQueue queue.Queuer) (*http.Server, error) {

	app := newApplicationHandler(msgRepo, appRepo, orgRepo, scheduleQueue)

	srv := &http.Server{
		Handler:      buildRoutes(app),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
	}

	tlsCfg := cfg.Server.HTTP.TLS
	// if the tls configuration options are empty, return here
	if tlsCfg == (config.TLSConfig{}) {
		return srv, nil
	}

	var xPool *x509.CertPool
	var err error

	// if a CA certificate file is provided, use it.
	// else, use the default system cert pool
	if tlsCfg.CAFile != "" {
		CACert, err := ioutil.ReadFile(tlsCfg.CAFile)
		if err != nil {
			return nil, err
		}

		xPool = x509.NewCertPool()
		if !xPool.AppendCertsFromPEM(CACert) {
			return nil, errors.New("failed to add ca cert file to cert pool")
		}
	} else {
		xPool, err = x509.SystemCertPool()
		if err != nil {
			return nil, errors.New("failed to load system cert pool")
		}
	}

	var cert tls.Certificate

	// if key file passphrase isn't empty, attempt to decrypt the private key
	// else just load the pair directly
	if tlsCfg.KeyFilePassphrase != "" {
		ce, err := withPassphrase(tlsCfg.CertFile, tlsCfg.KeyFile, []byte(tlsCfg.KeyFilePassphrase))
		if err != nil {
			return nil, err
		}
		cert = *ce
	} else {
		cert, err = tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
		if err != nil {
			return nil, err
		}
	}

	TLSConfig := &tls.Config{
		RootCAs:      xPool,
		Certificates: []tls.Certificate{cert},
	}

	// if the hostname is empty, make the server to skip verifying the hostname
	if tlsCfg.Hostname != "" {
		TLSConfig.ServerName = tlsCfg.Hostname
	} else {
		log.Warnf("no tls hostname provided, convoy will skip verifying the hostname provided by clients")
		TLSConfig.InsecureSkipVerify = true
	}

	srv.TLSConfig = TLSConfig
	prometheus.MustRegister(requestDuration)

	return srv, nil
}

// withPassphrase takes .key and .crt file paths, decodes the .key file with the give passphrase
// and constructs a tls.Certificate with the .crt file and the decoded .key file
func withPassphrase(pathToCert string, pathToKey string, password []byte) (*tls.Certificate, error) {

	keyFile, err := ioutil.ReadFile(pathToKey)
	if err != nil {
		return nil, err
	}

	certFile, err := ioutil.ReadFile(pathToCert)
	if err != nil {
		return nil, err
	}

	keyBlock, _ := pem.Decode(keyFile)

	// Decrypt key
	keyDER, err := pemutil.DecryptPEMBlock(keyBlock, password)
	if err != nil {
		return nil, err
	}

	keyBlock.Bytes = keyDER // Update keyBlock with the plaintext bytes
	keyBlock.Headers = nil  //clear the now obsolete headers.

	// Turn the key back into PEM format, so we can leverage tls.X509KeyPair,
	// which will deal with the intricacies of error handling, different key
	// types, certificate chains, etc.
	cert, err := tls.X509KeyPair(certFile, pem.EncodeToMemory(keyBlock))
	if err != nil {
		return nil, err
	}
	return &cert, nil
}
