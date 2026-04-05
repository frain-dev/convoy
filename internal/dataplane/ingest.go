package dataplane

import (
	"context"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/endpoints"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/sources"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func StartIngest(ctx context.Context, opts RuntimeOpts, cfg config.Configuration) error {
	sourceRepo := sources.New(opts.Logger, opts.DB)
	projectRepo := projects.New(opts.Logger, opts.DB)
	endpointRepo := endpoints.New(opts.Logger, opts.DB)
	configRepo := configuration.New(opts.Logger, opts.DB)

	lo := opts.Logger

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
		opts.Logger.Error("Failed to load configuration", "error", err)
	}

	var host string
	if instCfg != nil {
		host = instCfg.UID
	}

	rateLimiter, err := limiter.NewLimiter(cfg)
	if err != nil {
		return err
	}

	ingest, err := pubsub.NewIngest(ctx, sourceTable, opts.Queue, lo, rateLimiter, opts.Licenser, host, endpointRepo)
	if err != nil {
		return err
	}

	go ingest.Run()

	opts.Logger.Info("Starting Convoy Ingester")

	return nil
}
