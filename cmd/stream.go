package main

import (
	convoyMiddleware "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/internal/pkg/server"
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

			endpointRepo := cm.NewEndpointRepo(a.store)
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
				EndpointRepo:      endpointRepo,
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
				EndpointRepo: endpointRepo,
				GroupRepo:    groupRepo,
				Cache:        a.cache,
			})

			handler := socket.BuildRoutes(h, r, m)

			if c.Server.HTTP.SocketPort != 0 {
				socketPort = c.Server.HTTP.SocketPort
			}

			srv := server.NewServer(socketPort, func() {
				h.Stop()
			})

			srv.SetHandler(handler)

			log.Infof("Stream server running on port %v", socketPort)
			srv.Listen()
		},
	}

	cmd.Flags().Uint32Var(&socketPort, "socket-port", 5008, "Socket port")
	return cmd
}
