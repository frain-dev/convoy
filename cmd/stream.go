package main

import (
	convoyMiddleware "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/internal/pkg/socket"
	"github.com/frain-dev/convoy/pkg/log"

	cm "github.com/frain-dev/convoy/datastore/mongo"

	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/spf13/cobra"
)

func addStreamCommand(a *app) *cobra.Command {
	var socketPort uint32
	var logLevel string

	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Start a websocket server to pipe events to another convoy instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Get()
			if err != nil {
				a.logger.WithError(err).Fatal("failed to initialize realm chain")
				return err
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
				a.logger.WithError(err).Fatal("failed to initialize realm chain")
				return err
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

			lo := a.logger.(*log.Logger)
			lo.SetPrefix("socket server")

			lvl, err := log.ParseLevel(c.Logger.Level)
			if err != nil {
				return err
			}
			lo.SetLevel(lvl)

			m := convoyMiddleware.NewMiddleware(&convoyMiddleware.CreateMiddleware{
				Logger:    lo,
				AppRepo:   appRepo,
				GroupRepo: groupRepo,
				Cache:     a.cache,
			})

			handler := socket.BuildRoutes(h, r, m)

			if c.Server.HTTP.SocketPort != 0 {
				socketPort = c.Server.HTTP.SocketPort
			}

			srv := server.NewServer(socketPort, func() {
				h.Stop()
			})

			srv.SetHandler(handler)

			a.logger.Infof("Stream server running on port %v", socketPort)
			srv.Listen()

			return nil
		},
	}

	cmd.Flags().Uint32Var(&socketPort, "socket-port", 5008, "Socket port")
	cmd.Flags().StringVar(&logLevel, "log-level", "error", "stream log level")
	return cmd
}
