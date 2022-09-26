package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	convoyMiddleware "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/internal/pkg/socket"

	cm "github.com/frain-dev/convoy/datastore/mongo"

	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addStreamCommand(a *app) *cobra.Command {
	var socketPort uint32

	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Start a websocket server to pipe events to another convoy instance",
		Run: func(cmd *cobra.Command, args []string) {
			c, err := config.Get()
			if err != nil {
				log.WithError(err).Fatal("failed to initialize realm chain")
			}

			appRepo := cm.NewApplicationRepo(a.store)
			eventDeliveryRepo := cm.NewEventDeliveryRepository(a.store)
			sourceRepo := cm.NewSourceRepo(a.store)
			subRepo := cm.NewSubscriptionRepo(a.store)
			deviceRepo := cm.NewDeviceRepository(a.store)
			groupRepo := cm.NewGroupRepo(a.store)
			apiKeyRepo := cm.NewApiKeyRepo(a.store)

			// enable only the native auth realm
			authCfg := &config.AuthConfiguration{
				Native: config.NativeRealmOptions{Enabled: true},
			}

			err = realm_chain.Init(authCfg, apiKeyRepo, nil, nil)
			if err != nil {
				log.WithError(err).Fatal("failed to initialize realm chain")
			}

			r := &socket.Repo{
				AppRepo:           appRepo,
				DeviceRepo:        deviceRepo,
				SubscriptionRepo:  subRepo,
				SourceRepo:        sourceRepo,
				EventDeliveryRepo: eventDeliveryRepo,
			}

			h := socket.NewHub()
			go h.StartRegister()
			go h.StartUnregister()
			go h.StartEventWatcher()
			go h.StartEventSender()
			go h.StartClientStatusWatcher()

			m := convoyMiddleware.NewMiddleware(&convoyMiddleware.CreateMiddleware{
				AppRepo:   appRepo,
				GroupRepo: groupRepo,
				Cache:     a.cache,
			})

			router := socket.BuildRoutes(h, r, m)

			if c.Server.HTTP.SocketPort != 0 {
				socketPort = c.Server.HTTP.SocketPort
			}

			srv := &http.Server{
				Handler: router,
				Addr:    fmt.Sprintf(":%d", socketPort),
			}

			go func() {
				//service connections
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.WithError(err).Fatal("failed to listen")
				}
			}()

			log.Infof("Worker running on port %v", socketPort)
			gracefulShutdown(srv, h)
		},
	}

	cmd.Flags().Uint32Var(&socketPort, "socket-port", 5008, "Socket port")
	return cmd
}

func gracefulShutdown(srv *http.Server, hub *socket.Hub) {
	// Wait for interrupt signal to gracepfully shutdown the server with a timeout of 10 seconds
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
