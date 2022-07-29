package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	convoyMiddleware "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/services"
	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addStreamCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Start a websocket server to pipe events to another convoy instance",
		Run: func(cmd *cobra.Command, args []string) {
			// enable only the native auth realm
			authCfg := &config.AuthConfiguration{
				Native: config.NativeRealmOptions{Enabled: true},
			}
			err := realm_chain.Init(authCfg, a.apiKeyRepo, nil, nil)
			if err != nil {
				log.WithError(err).Fatal("failed to initialize realm chain")
			}

			hub := services.NewHub(a.deviceRepo, a.subRepo, a.sourceRepo, a.applicationRepo, watchCollection)
			go hub.StartRegister()
			go hub.StartUnregister()
			go hub.StartEventWatcher()
			go hub.StartEventSender()

			m := convoyMiddleware.NewMiddleware(&convoyMiddleware.CreateMiddleware{
				AppRepo:   a.applicationRepo,
				GroupRepo: a.groupRepo,
				Cache:     a.cache,
			})

			router := chi.NewRouter()
			router.Use(middleware.Recoverer)
			router.Route("/stream", func(streamRouter chi.Router) {
				streamRouter.Use(
					m.RequireAuth(),
					m.RequireGroup(),
					m.RequireAppID(),
					m.RequireAppPortalApplication(),
				)

				streamRouter.Get("/listen", hub.ListenHandler)
				streamRouter.Post("/login", hub.LoginHandler)
			})

			srv := &http.Server{
				Handler: router,
				Addr:    fmt.Sprintf(":%d", 5008),
			}

			go func() {
				//service connections
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.WithError(err).Fatal("failed to listen")
				}
			}()

			gracefulShutdown(srv, hub)
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	return cmd
}

func gracefulShutdown(srv *http.Server, hub *services.Hub) {
	//Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	hub.Stop()

	log.Info("Stopping websocket server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.WithError(err).Fatal("Server Shutdown")
	}

	log.Info("Websocket server exiting")

	time.Sleep(2 * time.Second) // allow all websocket connections close themselves
}
