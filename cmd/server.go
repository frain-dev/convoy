package main

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/server"
	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addServerCommand(a *app) *cobra.Command {

	var withWorkers bool
	var configFile string
	var redis string
	var dbDsn string
	var env string
	var baseUrl string
	var sentry string
	var limiter string
	var cache string
	var queue string
	var logger string
	var logLevel string
	var sslKeyFile string
	var sslCertFile string
	var retryStrategy string
	var signatureHash string
	var signatureHeader string
	var smtpProvider string
	var smtpUrl string
	var smtpUsername string
	var smtpPassword string
	var smtpReplyTo string
	var smtpFrom string
	var newReplicApp string
	var newReplicKey string
	var apiKeyAuthConfig string
	var basicAuthConfig string

	var ssl bool
	var requireAuth bool
	var disableEndpoint bool
	var multipleTenants bool
	var nativeRealmEnabled bool
	var newReplicTracerEnabled bool
	var newReplicConfigEnabled bool

	var port int32
	var smtpPort int32
	var workerPort int32
	var retryLimit int64
	var retryInterval int64

	cmd := &cobra.Command{
		Use:     "server",
		Aliases: []string{"serve", "s"},
		Short:   "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}

			err = StartConvoyServer(a, cfg, withWorkers)

			if err != nil {
				log.Printf("Error starting convoy server: %v", err)
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "./convoy.json", "Configuration file for convoy")
	cmd.Flags().StringVar(&redis, "redis", "", "Redis DSN")
	cmd.Flags().StringVar(&apiKeyAuthConfig, "api-auth", "", "API-Key authentication credentials")
	cmd.Flags().StringVar(&basicAuthConfig, "basic-auth", "", "Basic authentication credentials")
	cmd.Flags().StringVar(&dbDsn, "db", "", "Database DSN")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level")
	cmd.Flags().StringVar(&logger, "logger", "info", "Logger")
	cmd.Flags().StringVar(&env, "env", "development", "Convoy environment")
	cmd.Flags().StringVar(&baseUrl, "base-url", "", "Base Url - Used for the app portal")
	cmd.Flags().StringVar(&queue, "queue", "redis", "Queue Provider (\"redis\" or \"in-memory\")")
	cmd.Flags().StringVar(&cache, "cache", "redis", "Cache Provider (\"redis\" or \"in-memory\")")
	cmd.Flags().StringVar(&limiter, "limiter", "redis", "Rate limiter provider (\"redis\" or \"in-memory\")")
	cmd.Flags().StringVar(&sentry, "sentry", "", "Sentry DSN")
	cmd.Flags().StringVar(&sslCertFile, "ssl-cert-file", "", "SSL certificate file")
	cmd.Flags().StringVar(&sslKeyFile, "ssl-key-file", "", "SSL key file")
	cmd.Flags().StringVar(&retryStrategy, "retry-strategy", "", "Endpoint retry strategy")
	cmd.Flags().StringVar(&signatureHash, "signature-hash", "", "Application signature hash")
	cmd.Flags().StringVar(&signatureHeader, "signature-header", "", "Application signature header")
	cmd.Flags().StringVar(&smtpProvider, "smtp-provider", "", "SMTP provider")
	cmd.Flags().StringVar(&smtpUrl, "smtp-url", "", "SMTP provider URL")
	cmd.Flags().StringVar(&smtpUsername, "smtp-username", "", "SMTP authentication username")
	cmd.Flags().StringVar(&smtpPassword, "smtp-password", "", "SMTP authentication password")
	cmd.Flags().StringVar(&smtpFrom, "smtp-from", "", "Sender email address")
	cmd.Flags().StringVar(&smtpReplyTo, "smtp-reply-to", "", "Email address to reply to")
	cmd.Flags().StringVar(&newReplicApp, "new-relic-app", "", "NewRelic application name")
	cmd.Flags().StringVar(&newReplicKey, "new-relic-key", "", "NewRelic application license key")

	cmd.Flags().BoolVar(&ssl, "ssl", false, "Configure SSL")
	cmd.Flags().BoolVar(&requireAuth, "auth", false, "Require authentication")
	cmd.Flags().BoolVarP(&withWorkers, "with-workers", "w", true, "Should run workers")
	cmd.Flags().BoolVar(&nativeRealmEnabled, "native", false, "Enable native-realm authentication")
	cmd.Flags().BoolVar(&disableEndpoint, "disable-endpoint", false, "Disable all application endpoints")
	cmd.Flags().BoolVar(&newReplicConfigEnabled, "new-relic-config-enabled", false, "Enable new-relic config")
	cmd.Flags().BoolVar(&multipleTenants, "multi-tenant", false, "Start convoy in single- or multi-tenant mode")
	cmd.Flags().BoolVar(&newReplicTracerEnabled, "new-relic-tracer-enabled", false, "Enable new-relic distributed tracer")

	cmd.Flags().Int32Var(&port, "port", 0, "Server port")
	cmd.Flags().Int32Var(&smtpPort, "smtp-port", 0, "Server port")
	cmd.Flags().Int32Var(&workerPort, "worker-port", 0, "Worker port")
	cmd.Flags().Int64Var(&retryLimit, "retry-limit", 0, "Endpoint retry limit")
	cmd.Flags().Int64Var(&retryInterval, "retry-interval", 0, "Endpoint retry interval")

	return cmd
}

func StartConvoyServer(a *app, cfg config.Configuration, withWorkers bool) error {
	start := time.Now()
	log.Info("Starting Convoy server...")

	if util.IsStringEmpty(string(cfg.GroupConfig.Signature.Header)) {
		cfg.GroupConfig.Signature.Header = config.DefaultSignatureHeader
		log.Warnf("signature header is blank. setting default %s", config.DefaultSignatureHeader)
	}

	err := realm_chain.Init(&cfg.Auth, a.apiKeyRepo)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize realm chain")
	}

	if cfg.Server.HTTP.Port <= 0 {
		return errors.New("please provide the HTTP port in the convoy.json file")
	}

	srv := server.New(cfg,
		a.eventRepo,
		a.eventDeliveryRepo,
		a.applicationRepo,
		a.apiKeyRepo,
		a.groupRepo,
		a.eventQueue,
		a.logger,
		a.tracer,
		a.cache,
		a.limiter)

	if withWorkers {
		// register tasks.
		handler := task.ProcessEventDelivery(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo)
		if err := task.CreateTasks(a.groupRepo, handler); err != nil {
			log.WithError(err).Error("failed to register tasks")
			return err
		}

		worker.RegisterNewGroupTask(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo)

		log.Infof("Starting Convoy workers...")
		// register workers.
		ctx := context.Background()
		producer := worker.NewProducer(a.eventQueue)

		if cfg.Queue.Type != config.InMemoryQueueProvider {
			producer.Start(ctx)
		}

	}

	log.Infof("Started convoy server in %s", time.Since(start))

	httpConfig := cfg.Server.HTTP
	if httpConfig.SSL {
		log.Infof("Started server with SSL: cert_file: %s, key_file: %s", httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		return srv.ListenAndServeTLS(httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
	}

	log.Infof("Server running on port %v", cfg.Server.HTTP.Port)
	return srv.ListenAndServe()
}
