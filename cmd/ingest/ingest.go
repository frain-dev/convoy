package ingest

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

func AddIngestCommand(a *cli.App) *cobra.Command {
	var ingestPort uint32
	var logLevel string
	var interval int

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

			err = StartIngest(cmd.Context(), a, cfg, interval)
			if err != nil {
				return err
			}

			srv := server.NewServer(cfg.Server.HTTP.IngestPort, func() {})
			mux := chi.NewMux()
			mux.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{Registry: metrics.Reg()}))
			srv.SetHandler(mux)

			fmt.Printf("Starting Convoy Message Broker Ingester on port %v\n", cfg.Server.HTTP.IngestPort)
			srv.Listen()

			return nil
		},
	}

	cmd.Flags().Uint32Var(&ingestPort, "ingest-port", 0, "Ingest port")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "Log level")
	cmd.Flags().IntVar(&interval, "interval", 10, "the time interval, measured in seconds, at which the database should be polled for new pub sub sources")

	return cmd
}

func StartIngest(ctx context.Context, a *cli.App, cfg config.Configuration, interval int) error {
	sourceRepo := postgres.NewSourceRepo(a.DB)
	projectRepo := postgres.NewProjectRepo(a.DB)
	endpointRepo := postgres.NewEndpointRepo(a.DB)
	configRepo := postgres.NewConfigRepo(a.DB)

	lo := a.Logger.(*log.Logger)
	lo.SetPrefix("ingester")

	lvl, err := log.ParseLevel(cfg.Logger.Level)
	if err != nil {
		return err
	}

	lo.SetLevel(lvl)

	sourceLoader := pubsub.NewSourceLoader(endpointRepo, sourceRepo, projectRepo, lo)
	sourceTable := memorystore.NewTable(memorystore.OptionSyncer(sourceLoader))

	err = memorystore.DefaultStore.Register("sources", sourceTable)
	if err != nil {
		return err
	}

	go memorystore.DefaultStore.Sync(ctx, interval)

	instCfg, err := configRepo.LoadConfiguration(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to load configuration")
	}

	var host string
	if instCfg != nil {
		host = instCfg.UID
	}

	rateLimiter, err := limiter.NewLimiter(cfg)
	if err != nil {
		return err
	}

	ingest, err := pubsub.NewIngest(ctx, sourceTable, a.Queue, lo, rateLimiter, a.Licenser, host)
	if err != nil {
		return err
	}

	go ingest.Run()

	log.Println("Starting Convoy Ingester")

	return nil
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

	return c, nil
}
