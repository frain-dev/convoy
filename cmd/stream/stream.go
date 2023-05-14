package stream

import (
	"fmt"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/internal/pkg/socket"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/worker"
	"github.com/spf13/cobra"
)

func AddStreamCommand(a *cli.App) *cobra.Command {
	var socketPort uint32
	var logLevel string

	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Start a websocket server to pipe events to a convoy cli instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Get()
			if err != nil {
				a.Logger.WithError(err).Fatal("failed to initialize realm chain")
				return err
			}

			projectRepo := postgres.NewProjectRepo(a.DB)
			endpointRepo := postgres.NewEndpointRepo(a.DB)
			eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.DB)
			sourceRepo := postgres.NewSourceRepo(a.DB)
			subRepo := postgres.NewSubscriptionRepo(a.DB)
			deviceRepo := postgres.NewDeviceRepo(a.DB)
			apiKeyRepo := postgres.NewAPIKeyRepo(a.DB)
			userRepo := postgres.NewUserRepo(a.DB)
			orgMemberRepo := postgres.NewOrgMemberRepo(a.DB)

			// enable only the native auth realm
			authCfg := &config.AuthConfiguration{
				Native: config.NativeRealmOptions{Enabled: true},
			}

			err = realm_chain.Init(authCfg, apiKeyRepo, userRepo, nil)
			if err != nil {
				a.Logger.WithError(err).Fatal("failed to initialize realm chain")
				return err
			}

			r := &socket.Repo{
				OrgMemberRepository: orgMemberRepo,
				ProjectRepo:         projectRepo,
				EndpointRepo:        endpointRepo,
				DeviceRepo:          deviceRepo,
				SubscriptionRepo:    subRepo,
				SourceRepo:          sourceRepo,
				EventDeliveryRepo:   eventDeliveryRepo,
			}

			h := socket.NewHub()
			h.Start()

			lo := a.Logger.(*log.Logger)
			lo.SetPrefix("stream server")

			lvl, err := log.ParseLevel(c.Logger.Level)
			if err != nil {
				return err
			}
			lo.SetLevel(lvl)

			handler := socket.BuildRoutes(h, r)

			consumer := worker.NewConsumer(a.Queue, lo)
			consumer.RegisterHandlers(convoy.StreamCliEventsProcessor, h.EventDeliveryCLiHandler(r))

			// start worker
			fmt.Println("Registering Stream Server Consumer...")
			consumer.Start()

			if c.Server.HTTP.SocketPort != 0 {
				socketPort = c.Server.HTTP.SocketPort
			}

			srv := server.NewServer(socketPort, func() { h.Stop() })

			srv.SetHandler(handler)

			fmt.Printf("Stream server running on port %v\n", socketPort)
			srv.Listen()

			return nil
		},
	}

	cmd.Flags().Uint32Var(&socketPort, "socket-port", 5008, "Socket port")
	cmd.Flags().StringVar(&logLevel, "log-level", "error", "stream log level")
	return cmd
}
