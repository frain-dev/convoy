package agent

import (
	"context"
	"os"
	"os/signal"
	"time"

	workerSrv "github.com/frain-dev/convoy/cmd/worker"
	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/api"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm_chain"
	ingestSrv "github.com/frain-dev/convoy/cmd/ingest"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/spf13/cobra"
)

func AddAgentCommand(a *cli.App) *cobra.Command {
	var agentPort uint32
	var logLevel string
	var consumerPoolSize int
	var interval int

	var smtpSSL bool
	var smtpUsername string
	var smtpPassword string
	var smtpReplyTo string
	var smtpFrom string
	var smtpProvider string
	var executionMode string
	var smtpUrl string
	var smtpPort uint32

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Start agent instance",
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

			cfg, err := config.Get()
			if err != nil {
				a.Logger.WithError(err).Fatal("Failed to load configuration")
			}

			// start sync configuration from the database.
			go memorystore.DefaultStore.Sync(ctx, interval)

			err = workerSrv.StartWorker(ctx, a, cfg, interval)
			if err != nil {
				a.Logger.Errorf("Error starting data plane worker component, err: %v", err)
				return err
			}

			err = ingestSrv.StartIngest(cmd.Context(), a, cfg, interval)
			if err != nil {
				a.Logger.Errorf("Error starting data plane ingest component: %v", err)
				return err
			}

			err = startServerComponent(ctx, a)
			if err != nil {
				a.Logger.Errorf("Error starting data plane server component: %v", err)
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

	cmd.Flags().BoolVar(&smtpSSL, "smtp-ssl", false, "Enable SMTP SSL")
	cmd.Flags().StringVar(&smtpUsername, "smtp-username", "", "SMTP authentication username")
	cmd.Flags().StringVar(&smtpPassword, "smtp-password", "", "SMTP authentication password")
	cmd.Flags().StringVar(&smtpFrom, "smtp-from", "", "Sender email address")
	cmd.Flags().StringVar(&smtpReplyTo, "smtp-reply-to", "", "Email address to reply to")
	cmd.Flags().StringVar(&smtpProvider, "smtp-provider", "", "SMTP provider")
	cmd.Flags().StringVar(&smtpUrl, "smtp-url", "", "SMTP provider URL")
	cmd.Flags().Uint32Var(&smtpPort, "smtp-port", 0, "SMTP Port")

	cmd.Flags().Uint32Var(&agentPort, "port", 0, "Agent port")

	cmd.Flags().StringVar(&logLevel, "log-level", "", "Log level")
	cmd.Flags().IntVar(&consumerPoolSize, "consumers", -1, "Size of the consumers pool.")
	cmd.Flags().IntVar(&interval, "interval", 10, "the time interval, measured in seconds to update the in-memory store from the database")
	cmd.Flags().StringVar(&executionMode, "mode", "", "Execution Mode (one of events, retry and default)")

	return cmd
}

func startServerComponent(_ context.Context, a *cli.App) error {
	lo := a.Logger.(*log.Logger)
	lo.SetPrefix("agent")

	cfg, err := config.Get()
	if err != nil {
		lo.WithError(err).Fatal("Failed to load configuration")
	}

	start := time.Now()
	lo.Info("Starting Convoy data plane")

	apiKeyRepo := postgres.NewAPIKeyRepo(a.DB, a.Cache)
	userRepo := postgres.NewUserRepo(a.DB, a.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(a.DB, a.Cache)
	err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, a.Cache)
	if err != nil {
		lo.WithError(err).Fatal("failed to initialize realm chain")
	}

	flag := fflag.NewFFlag(&cfg)

	lvl, err := log.ParseLevel(cfg.Logger.Level)
	if err != nil {
		return err
	}
	lo.SetLevel(lvl)

	// start events handler
	srv := server.NewServer(cfg.Server.HTTP.AgentPort, func() {})

	evHandler, err := api.NewApplicationHandler(
		&types.APIOptions{
			FFlag:    flag,
			DB:       a.DB,
			Queue:    a.Queue,
			Logger:   lo,
			Cache:    a.Cache,
			Rate:     a.Rate,
			Redis:    a.Redis,
			Licenser: a.Licenser,
		})
	if err != nil {
		return err
	}

	srv.SetHandler(evHandler.BuildDataPlaneRoutes())

	lo.Infof("Started convoy server in %s", time.Since(start))

	httpConfig := cfg.Server.HTTP
	if httpConfig.SSL {
		a.Logger.Infof("Started server with SSL: cert_file: %s, key_file: %s", httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		srv.ListenAndServeTLS(httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		return nil
	}

	lo.Infof("Starting Convoy Agent on port %v", cfg.Server.HTTP.AgentPort)

	go func() {
		srv.Listen()
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

	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(logLevel) {
		c.Logger.Level = logLevel
	}

	// CONVOY_WORKER_POOL_SIZE
	consumerPoolSize, err := cmd.Flags().GetInt("consumers")
	if err != nil {
		return nil, err
	}

	if consumerPoolSize >= 0 {
		c.ConsumerPoolSize = consumerPoolSize
	}

	// CONVOY_SMTP_PROVIDER
	smtpProvider, err := cmd.Flags().GetString("smtp-provider")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpProvider) {
		c.SMTP.Provider = smtpProvider
	}

	// CONVOY_SMTP_URL
	smtpUrl, err := cmd.Flags().GetString("smtp-url")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpUrl) {
		c.SMTP.URL = smtpUrl
	}

	// CONVOY_SMTP_USERNAME
	smtpUsername, err := cmd.Flags().GetString("smtp-username")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpUsername) {
		c.SMTP.Username = smtpUsername
	}

	// CONVOY_SMTP_PASSWORD
	smtpPassword, err := cmd.Flags().GetString("smtp-password")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpPassword) {
		c.SMTP.Password = smtpPassword
	}

	// CONVOY_SMTP_FROM
	smtpFrom, err := cmd.Flags().GetString("smtp-from")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpFrom) {
		c.SMTP.From = smtpFrom
	}

	// CONVOY_SMTP_REPLY_TO
	smtpReplyTo, err := cmd.Flags().GetString("smtp-reply-to")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpReplyTo) {
		c.SMTP.ReplyTo = smtpReplyTo
	}

	// CONVOY_SMTP_PORT
	smtpPort, err := cmd.Flags().GetUint32("smtp-port")
	if err != nil {
		return nil, err
	}

	if smtpPort != 0 {
		c.SMTP.Port = smtpPort
	}

	// CONVOY_WORKER_EXECUTION_MODE
	executionMode, err := cmd.Flags().GetString("mode")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(executionMode) {
		c.WorkerExecutionMode = config.ExecutionMode(executionMode)
	}

	return c, nil
}
