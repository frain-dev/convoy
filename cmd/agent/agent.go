package agent

import (
	"context"
	"github.com/frain-dev/convoy/internal/telemetry"
	"os"
	"os/signal"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/spf13/cobra"
)

func AddAgentCommand(a *cli.App) *cobra.Command {
	var port uint32
	var interval int
	var consumerPoolSize int

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Start agent instance",
		Annotations: map[string]string{
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, os.Interrupt)

			defer func() {
				signal.Stop(quit)
				cancel()
			}()

			// override config with cli flags
			cliConfig, err := buildAgentCliConfiguration(cmd)
			if err != nil {
				return err
			}

			if err = config.Override(cliConfig); err != nil {
				return err
			}

			err = startServerComponent(ctx, a)
			if err != nil {
				a.Logger.Errorf("Error starting data plane server component: %v", err)
				return err
			}

			err = startIngestComponent(ctx, a, interval)
			if err != nil {
				a.Logger.Errorf("Error starting data plane ingest component: %v", err)
				return err
			}

			err = startWorkerComponent(ctx, a)
			if err != nil {
				a.Logger.Errorf("Error starting data plane worker component, err: %v", err)
				return err
			}

			select {
			case <-quit:
				return nil
			case <-ctx.Done():
			}

			return ctx.Err()
		},
	}

	cmd.Flags().Uint32Var(&port, "port", 0, "Agent port")
	cmd.Flags().IntVar(&interval, "interval", 300, "the time interval, measured in seconds, at which the database should be polled for new pub sub sources")
	cmd.Flags().IntVar(&consumerPoolSize, "consumers", -1, "Size of the consumers pool.")

	return cmd
}

func startServerComponent(ctx context.Context, a *cli.App) error {
	cfg, err := config.Get()
	if err != nil {
		a.Logger.WithError(err).Fatal("Failed to load configuration")
	}

	start := time.Now()
	a.Logger.Info("Starting Convoy data plane ...")

	apiKeyRepo := postgres.NewAPIKeyRepo(a.DB, a.Cache)
	userRepo := postgres.NewUserRepo(a.DB, a.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(a.DB, a.Cache)
	err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, a.Cache)
	if err != nil {
		a.Logger.WithError(err).Fatal("failed to initialize realm chain")
	}

	flag := fflag.NewFFlag()
	if err != nil {
		a.Logger.WithError(err).Fatal("failed to create fflag controller")
	}

	lo := a.Logger.(*log.Logger)
	lo.SetPrefix("api server")

	lvl, err := log.ParseLevel(cfg.Logger.Level)
	if err != nil {
		return err
	}
	lo.SetLevel(lvl)

	// start events handler
	srv := server.NewServer(cfg.Server.HTTP.AgentPort, func() {})

	evHandler, err := api.NewApplicationHandler(
		&types.APIOptions{
			FFlag:  flag,
			DB:     a.DB,
			Queue:  a.Queue,
			Logger: lo,
			Cache:  a.Cache,
		})
	if err != nil {
		return err
	}

	srv.SetHandler(evHandler.BuildDataPlaneRoutes())

	a.Logger.Infof("Started convoy server in %s", time.Since(start))

	httpConfig := cfg.Server.HTTP
	if httpConfig.SSL {
		a.Logger.Infof("Started server with SSL: cert_file: %s, key_file: %s", httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		srv.ListenAndServeTLS(httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		return nil
	}

	a.Logger.Infof("Server running on port %v", cfg.Server.HTTP.AgentPort)

	go func() {
		srv.Listen()
	}()

	return nil
}

func startIngestComponent(ctx context.Context, a *cli.App, interval int) error {
	// start message broker ingest.
	sourceRepo := postgres.NewSourceRepo(a.DB, a.Cache)
	projectRepo := postgres.NewProjectRepo(a.DB, a.Cache)
	endpointRepo := postgres.NewEndpointRepo(a.DB, a.Cache)

	sourceLoader := pubsub.NewSourceLoader(endpointRepo, sourceRepo, projectRepo, a.Logger)
	sourceTable := memorystore.NewTable(memorystore.OptionSyncer(sourceLoader))

	err := memorystore.DefaultStore.Register("sources", sourceTable)
	if err != nil {
		return err
	}

	go memorystore.DefaultStore.Sync(ctx, interval)

	ingest, err := pubsub.NewIngest(ctx, sourceTable, a.Queue, a.Logger)
	if err != nil {
		return err
	}

	go ingest.Run()

	return nil
}

func startWorkerComponent(ctx context.Context, a *cli.App) error {
	cfg, err := config.Get()
	if err != nil {
		a.Logger.WithError(err).Fatal("Failed to load configuration")
		return err
	}

	// register worker.
	consumer := worker.NewConsumer(ctx, cfg.ConsumerPoolSize, a.Queue, a.Logger)
	projectRepo := postgres.NewProjectRepo(a.DB, a.Cache)
	metaEventRepo := postgres.NewMetaEventRepo(a.DB, a.Cache)
	endpointRepo := postgres.NewEndpointRepo(a.DB, a.Cache)
	eventRepo := postgres.NewEventRepo(a.DB, a.Cache)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.DB, a.Cache)
	subRepo := postgres.NewSubscriptionRepo(a.DB, a.Cache)
	deviceRepo := postgres.NewDeviceRepo(a.DB, a.Cache)
	configRepo := postgres.NewConfigRepo(a.DB)

	rateLimiter := limiter.NewLimiter(a.DB)

	counter := &telemetry.EventsCounter{}

	pb := telemetry.NewposthogBackend()
	mb := telemetry.NewmixpanelBackend()

	configuration, err := configRepo.LoadConfiguration(context.Background())
	if err != nil {
		a.Logger.WithError(err).Fatal("Failed to instance configuration")
		return err
	}

	newTelemetry := telemetry.NewTelemetry(a.Logger.(*log.Logger), configuration,
		telemetry.OptionTracker(counter),
		telemetry.OptionBackend(pb),
		telemetry.OptionBackend(mb))

	consumer.RegisterHandlers(convoy.EventProcessor, task.ProcessEventDelivery(
		endpointRepo,
		eventDeliveryRepo,
		projectRepo,
		a.Queue,
		rateLimiter), newTelemetry)

	consumer.RegisterHandlers(convoy.CreateEventProcessor, task.ProcessEventCreation(
		endpointRepo,
		eventRepo,
		projectRepo,
		eventDeliveryRepo,
		a.Queue,
		subRepo,
		deviceRepo), newTelemetry)

	consumer.RegisterHandlers(convoy.CreateDynamicEventProcessor, task.ProcessDynamicEventCreation(
		endpointRepo,
		eventRepo,
		projectRepo,
		eventDeliveryRepo,
		a.Queue,
		subRepo,
		deviceRepo), newTelemetry)

	consumer.RegisterHandlers(convoy.MetaEventProcessor, task.ProcessMetaEvent(projectRepo, metaEventRepo), nil)

	consumer.RegisterHandlers(convoy.CreateBroadcastEventProcessor, task.ProcessBroadcastEventCreation(
		a.DB,
		endpointRepo,
		eventRepo,
		projectRepo,
		eventDeliveryRepo,
		a.Queue,
		subRepo,
		deviceRepo), newTelemetry)

	ticker := time.NewTicker(time.Second * 10)
	go task.QueueStuckEventDeliveries(ctx, ticker, eventDeliveryRepo, a.Queue)

	go func() {
		consumer.Start()
		<-ctx.Done()
	}()

	return nil
}

func buildAgentCliConfiguration(cmd *cobra.Command) (*config.Configuration, error) {
	c := &config.Configuration{}

	// PORT
	port, err := cmd.Flags().GetUint32("port")
	if err != nil {
		return nil, err
	}

	if port != 0 {
		c.Server.HTTP.AgentPort = port
	}

	return c, nil
}
