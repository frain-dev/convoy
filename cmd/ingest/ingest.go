package ingest

import (
	"context"
	"log/slog"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/sources"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func StartIngest(ctx context.Context, a *cli.App, cfg config.Configuration, interval int) error {
	sourceRepo := sources.New(a.Logger, a.DB)
	projectRepo := projects.New(a.Logger, a.DB)
	endpointRepo := postgres.NewEndpointRepo(a.DB)
	configRepo := configuration.New(a.Logger, a.DB)

	lo := a.Logger

	_, err := log.ParseLevel(cfg.Logger.Level)
	if err != nil {
		return err
	}

	sourceLoader := pubsub.NewSourceLoader(endpointRepo, sourceRepo, projectRepo, lo)
	sourceTable := memorystore.NewTable(memorystore.OptionSyncer(sourceLoader))

	err = memorystore.DefaultStore.Register("sources", sourceTable)
	if err != nil {
		return err
	}

	instCfg, err := configRepo.LoadConfiguration(ctx)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
	}

	var host string
	if instCfg != nil {
		host = instCfg.UID
	}

	rateLimiter, err := limiter.NewLimiter(cfg)
	if err != nil {
		return err
	}

	ingest, err := pubsub.NewIngest(ctx, sourceTable, a.Queue, lo, rateLimiter, a.Licenser, host, endpointRepo)
	if err != nil {
		return err
	}

	go ingest.Run()

	slog.Info("Starting Convoy Ingester")

	return nil
}
