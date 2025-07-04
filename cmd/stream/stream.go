package stream

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/queue"
	redisQueue "github.com/frain-dev/convoy/queue/redis"

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

	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Start a websocket server to pipe events to a convoy cli instance",
		Annotations: map[string]string{
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			// override config with cli flags
			cliConfig, err := buildCliFlagConfiguration(cmd)
			if err != nil {
				return err
			}

			if err = config.Override(cliConfig); err != nil {
				return err
			}

			cfg, err := config.Get()
			if err != nil {
				a.Logger.Errorf("Failed to retrieve config: %v", err)
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
			portalLinkRepo := postgres.NewPortalLinkRepo(a.DB)

			// enable only the native auth realm
			authCfg := &config.AuthConfiguration{
				Native: config.NativeRealmOptions{Enabled: true},
			}

			err = realm_chain.Init(authCfg, apiKeyRepo, userRepo, portalLinkRepo, nil)
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

			lo := a.Logger.(*log.Logger)
			lo.SetPrefix("stream server")

			h := socket.NewHub()
			h.Start(context.Background())

			handler := socket.BuildRoutes(r)

			redis, err := rdb.NewClient(cfg.Redis.BuildDsn())
			if err != nil {
				return err
			}
			queueNames := map[string]int{
				string(convoy.StreamQueue): 1,
			}

			opts := queue.QueueOptions{
				Names:             queueNames,
				RedisClient:       redis,
				RedisAddress:      cfg.Redis.BuildDsn(),
				Type:              string(config.RedisQueueProvider),
				PrometheusAddress: cfg.Prometheus.Dsn,
			}
			q := redisQueue.NewQueue(opts)

			lvl, err := log.ParseLevel(cfg.Logger.Level)
			if err != nil {
				return err
			}

			consumer := worker.NewConsumer(ctx, 100, q, lo, lvl)
			consumer.RegisterHandlers(convoy.StreamCliEventsProcessor, h.EventDeliveryCLiHandler(r), nil)

			// start worker
			fmt.Println("Registering Stream Server Consumer...")
			consumer.Start()

			if cfg.Server.HTTP.SocketPort != 0 {
				socketPort = cfg.Server.HTTP.SocketPort
			}

			srv := server.NewServer(socketPort, func() { h.Stop() })

			srv.SetHandler(handler)

			fmt.Printf("Stream server running on port %v\n", socketPort)
			srv.Listen()

			return nil
		},
	}

	cmd.Flags().Uint32Var(&socketPort, "socket-port", 5008, "Socket port")

	return cmd
}

func buildCliFlagConfiguration(_ *cobra.Command) (*config.Configuration, error) {
	c := &config.Configuration{}

	return c, nil
}
