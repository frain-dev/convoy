package stream

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/util"

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

	var newRelicApp string
	var newRelicKey string
	var newRelicTracerEnabled bool
	var newRelicConfigEnabled bool

	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Start a websocket server to pipe events to a convoy cli instance",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			lo := a.Logger.(*log.Logger)
			lo.SetPrefix("stream server")

			lvl, err := log.ParseLevel(cfg.Logger.Level)
			if err != nil {
				return err
			}
			lo.SetLevel(lvl)

			h := socket.NewHub()
			h.Start(context.Background())

			handler := socket.BuildRoutes(r)

			consumer := worker.NewConsumer(a.Queue, lo)
			consumer.RegisterHandlers(convoy.StreamCliEventsProcessor, h.EventDeliveryCLiHandler(r))

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
	cmd.Flags().StringVar(&logLevel, "log-level", "error", "stream log level")

	cmd.Flags().BoolVar(&newRelicConfigEnabled, "new-relic-config-enabled", false, "Enable new-relic config")
	cmd.Flags().BoolVar(&newRelicTracerEnabled, "new-relic-tracer-enabled", false, "Enable new-relic distributed tracer")
	cmd.Flags().StringVar(&newRelicApp, "new-relic-app", "", "NewRelic application name")
	cmd.Flags().StringVar(&newRelicKey, "new-relic-key", "", "NewRelic application license key")

	return cmd
}

func buildCliFlagConfiguration(cmd *cobra.Command) (*config.Configuration, error) {
	c := &config.Configuration{}

	// CONVOY_NEWRELIC_APP_NAME
	newReplicApp, err := cmd.Flags().GetString("new-relic-app")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(newReplicApp) {
		c.Tracer.NewRelic.AppName = newReplicApp
	}

	// CONVOY_NEWRELIC_LICENSE_KEY
	newReplicKey, err := cmd.Flags().GetString("new-relic-key")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(newReplicKey) {
		c.Tracer.NewRelic.LicenseKey = newReplicKey
	}

	// CONVOY_NEWRELIC_CONFIG_ENABLED
	isNRCESet := cmd.Flags().Changed("new-relic-config-enabled")
	if isNRCESet {
		newReplicConfigEnabled, err := cmd.Flags().GetBool("new-relic-config-enabled")
		if err != nil {
			return nil, err
		}

		c.Tracer.NewRelic.ConfigEnabled = newReplicConfigEnabled
	}

	// CONVOY_NEWRELIC_DISTRIBUTED_TRACER_ENABLED
	isNRTESet := cmd.Flags().Changed("new-relic-tracer-enabled")
	if isNRTESet {
		newReplicTracerEnabled, err := cmd.Flags().GetBool("new-relic-tracer-enabled")
		if err != nil {
			return nil, err
		}

		c.Tracer.NewRelic.DistributedTracerEnabled = newReplicTracerEnabled
	}

	return c, nil
}
