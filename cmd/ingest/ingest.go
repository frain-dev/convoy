package ingest

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
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

			sourceRepo := postgres.NewSourceRepo(a.DB, a.Cache)
			projectRepo := postgres.NewProjectRepo(a.DB, a.Cache)
			endpointRepo := postgres.NewEndpointRepo(a.DB, a.Cache)

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

			go memorystore.DefaultStore.Sync(cmd.Context(), interval)

			ingest, err := pubsub.NewIngest(cmd.Context(), sourceTable, a.Queue, lo)
			if err != nil {
				return err
			}

			go ingest.Run()

			srv := server.NewServer(cfg.Server.HTTP.IngestPort, func() {})
			srv.SetHandler(chi.NewMux())

			a.Logger.Info("Starting Convoy Message Broker Ingester...")

			srv.Listen()

			return nil
		},
	}

	cmd.Flags().Uint32Var(&ingestPort, "ingest-port", 5009, "Ingest port")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "ingest log level")
	cmd.Flags().IntVar(&interval, "interval", 10, "the time interval, measured in seconds, at which the database should be polled for new pub sub sources")

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

	return c, nil
}
