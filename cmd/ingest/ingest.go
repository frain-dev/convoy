package ingest

import (
	"context"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/spf13/cobra"
)

func AddIngestCommand(a *cli.App) *cobra.Command {
	var ingestPort uint32
	var logLevel string
	var interval int

	var newRelicApp string
	var newRelicKey string
	var newRelicTracerEnabled bool
	var newRelicConfigEnabled bool

	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Ingest webhook events from Pub/Sub streams",
		Annotations: map[string]string{
			"ShouldBootstrap": "false",
		},
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

			sourceRepo := postgres.NewSourceRepo(a.DB)
			projectRepo := postgres.NewProjectRepo(a.DB)
			endpointRepo := postgres.NewEndpointRepo(a.DB)

			lo := a.Logger.(*log.Logger)
			lo.SetPrefix("ingester")

			lvl, err := log.ParseLevel(cfg.Logger.Level)
			if err != nil {
				return err
			}

			lo.SetLevel(lvl)

			sourcePool := pubsub.NewSourcePool(lo)
			sourceLoader := pubsub.NewSourceLoader(endpointRepo, sourceRepo, projectRepo, a.Queue, sourcePool, lo)

			stop := make(chan struct{})
			go sourceLoader.Run(context.Background(), interval, stop)

			srv := server.NewServer(cfg.Server.HTTP.IngestPort, func() { stop <- struct{}{} })
			srv.SetHandler(chi.NewMux())

			srv.Listen()

			return nil
		},
	}

	cmd.Flags().Uint32Var(&ingestPort, "ingest-port", 5009, "Ingest port")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "ingest log level")
	cmd.Flags().IntVar(&interval, "interval", 300, "the time interval, measured in seconds, at which the database should be polled for new pub sub sources")
	cmd.Flags().BoolVar(&newRelicConfigEnabled, "new-relic-config-enabled", false, "Enable new-relic config")
	cmd.Flags().BoolVar(&newRelicTracerEnabled, "new-relic-tracer-enabled", false, "Enable new-relic distributed tracer")
	cmd.Flags().StringVar(&newRelicApp, "new-relic-app", "", "NewRelic application name")
	cmd.Flags().StringVar(&newRelicKey, "new-relic-key", "", "NewRelic application license key")

	return cmd
}

func buildCliFlagConfiguration(cmd *cobra.Command) (*config.Configuration, error) {
	c := &config.Configuration{}

	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(logLevel) {
		c.Logger.Level = logLevel
	}

	ingestPort, err := cmd.Flags().GetUint32("ingest-port")
	if err != nil {
		return nil, err
	}

	if ingestPort != 0 {
		c.Server.HTTP.IngestPort = ingestPort
	}

	c.Server.HTTP.IngestPort = ingestPort

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
