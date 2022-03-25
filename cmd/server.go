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

	var env string
	var baseUrl string
	var sentry string
	var limiter string
	var cache string
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
	var withWorkers bool
	var requireAuth bool
	var disableEndpoint bool
	var multipleTenants bool
	var nativeRealmEnabled bool
	var newReplicTracerEnabled bool
	var newReplicConfigEnabled bool

	var port uint32
	var smtpPort uint32
	var retryLimit uint64
	var workerPort uint32
	var retryInterval uint64
	var maxResponseSize uint64

	cmd := &cobra.Command{
		Use:     "server",
		Aliases: []string{"serve", "s"},
		Short:   "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Get()
			if err != nil {
				return err
			}

			// override config with cli flags
			err = loadServerConfigFromCliFlags(cmd, &c)
			if err != nil {
				return err
			}

			err = config.SetServerConfigDefaults(&c)
			if err != nil {
				return err
			}

			err = StartConvoyServer(a, c, withWorkers)

			if err != nil {
				log.Printf("Error starting convoy server: %v", err)
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&apiKeyAuthConfig, "api-auth", "", "API-Key authentication credentials")
	cmd.Flags().StringVar(&basicAuthConfig, "basic-auth", "", "Basic authentication credentials")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level")
	cmd.Flags().StringVar(&logger, "logger", "info", "Logger")
	cmd.Flags().StringVar(&env, "env", "development", "Convoy environment")
	cmd.Flags().StringVar(&baseUrl, "base-url", "", "Base Url - Used for the app portal")
	cmd.Flags().StringVar(&cache, "cache", "redis", `Cache Provider ("redis" or "in-memory")`)
	cmd.Flags().StringVar(&limiter, "limiter", "redis", `Rate limiter provider ("redis" or "in-memory")`)
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

	cmd.Flags().Uint32Var(&port, "port", 0, "Server port")
	cmd.Flags().Uint32Var(&smtpPort, "smtp-port", 0, "Server port")
	cmd.Flags().Uint32Var(&workerPort, "worker-port", 0, "Worker port")
	cmd.Flags().Uint64Var(&retryLimit, "retry-limit", 0, "Endpoint retry limit")
	cmd.Flags().Uint64Var(&maxResponseSize, "max-response-size", 0, "Max response size")
	cmd.Flags().Uint64Var(&retryInterval, "retry-interval", 0, "Endpoint retry interval")

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
		handler := task.ProcessEventDelivery(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo, a.limiter)
		if err := task.CreateTasks(a.groupRepo, handler); err != nil {
			log.WithError(err).Error("failed to register tasks")
			return err
		}

		worker.RegisterNewGroupTask(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo, a.limiter)

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

func loadServerConfigFromCliFlags(cmd *cobra.Command, c *config.Configuration) error {
	// CONVOY_ENV
	env, err := cmd.Flags().GetString("env")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(env) {
		c.Environment = env
	}

	// CONVOY_BASE_URL
	baseUrl, err := cmd.Flags().GetString("base-url")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(baseUrl) {
		c.BaseUrl = baseUrl
	}

	// CONVOY_SENTRY_DSN
	sentryDsn, err := cmd.Flags().GetString("sentry")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(sentryDsn) {
		c.Sentry.Dsn = sentryDsn
	}

	// CONVOY_MULTIPLE_TENANTS
	isMTSet := cmd.Flags().Changed("multi-tenant")
	if isMTSet {
		multipleTenants, err := cmd.Flags().GetBool("multi-tenant")
		if err != nil {
			return err
		}

		c.MultipleTenants = multipleTenants
	}

	// CONVOY_REDIS_DSN
	redis, err := cmd.Flags().GetString("redis")
	if err != nil {
		return err
	}

	// CONVOY_LIMITER_PROVIDER
	rateLimiter, err := cmd.Flags().GetString("limiter")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(rateLimiter) {
		c.Limiter.Type = config.LimiterProvider(rateLimiter)
		if rateLimiter == "redis" && !util.IsStringEmpty(redis) {
			c.Limiter.Redis.Dsn = redis
		}
	}

	// CONVOY_CACHE_PROVIDER
	cache, err := cmd.Flags().GetString("cache")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(cache) {
		c.Cache.Type = config.CacheProvider(cache)
		if cache == "redis" && !util.IsStringEmpty(redis) {
			c.Cache.Redis.Dsn = redis
		}
	}

	// CONVOY_LOGGER_LEVEL
	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(logLevel) {
		c.Logger.ServerLog.Level = logLevel
	}

	// CONVOY_LOGGER_PROVIDER
	logger, err := cmd.Flags().GetString("logger")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(logger) {
		c.Logger.Type = config.LoggerProvider(logger)
	}

	// SSL
	isSslSet := cmd.Flags().Changed("ssl")
	if isSslSet {
		ssl, err := cmd.Flags().GetBool("ssl")
		if err != nil {
			return err
		}

		c.Server.HTTP.SSL = ssl
	}

	// PORT
	port, err := cmd.Flags().GetUint32("port")
	if err != nil {
		return err
	}

	if port != 0 {
		c.Server.HTTP.Port = port
	}

	// WORKER_PORT
	workerPort, err := cmd.Flags().GetUint32("worker-port")
	if err != nil {
		return err
	}

	if workerPort != 0 {
		c.Server.HTTP.WorkerPort = workerPort
	}

	// CONVOY_SSL_KEY_FILE
	sslKeyFile, err := cmd.Flags().GetString("ssl-key-file")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(sslKeyFile) {
		c.Server.HTTP.SSLKeyFile = sslKeyFile
	}

	// CONVOY_SSL_CERT_FILE
	sslCertFile, err := cmd.Flags().GetString("ssl-cert-file")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(sslCertFile) {
		c.Server.HTTP.SSLCertFile = sslCertFile
	}

	// CONVOY_STRATEGY_TYPE
	retryStrategy, err := cmd.Flags().GetString("retry-strategy")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(retryStrategy) {
		c.GroupConfig.Strategy.Type = config.StrategyProvider(retryStrategy)
	}

	// CONVOY_SIGNATURE_HASH
	signatureHash, err := cmd.Flags().GetString("signature-hash")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(signatureHash) {
		c.GroupConfig.Signature.Hash = signatureHash
	}

	// CONVOY_SIGNATURE_HEADER
	signatureHeader, err := cmd.Flags().GetString("signature-header")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(signatureHeader) {
		c.GroupConfig.Signature.Header = config.SignatureHeaderProvider(signatureHeader)
	} else if util.IsStringEmpty(c.GroupConfig.Signature.Header.String()) {
		c.GroupConfig.Signature.Header = config.DefaultSignatureHeader
	}

	// CONVOY_DISABLE_ENDPOINT
	isDisableEndpointSet := cmd.Flags().Changed("disable-endpoint")
	if isDisableEndpointSet {
		disableEndpoint, err := cmd.Flags().GetBool("disable-endpoint")
		if err != nil {
			return err
		}

		c.GroupConfig.DisableEndpoint = disableEndpoint
	}

	// CONVOY_INTERVAL_SECONDS
	retryInterval, err := cmd.Flags().GetUint64("retry-interval")
	if err != nil {
		return err
	}

	if retryInterval != 0 {
		c.GroupConfig.Strategy.Default.IntervalSeconds = retryInterval
	}

	// CONVOY_RETRY_LIMIT
	retryLimit, err := cmd.Flags().GetUint64("retry-limit")
	if err != nil {
		return err
	}

	if retryLimit != 0 {
		c.GroupConfig.Strategy.Default.RetryLimit = retryLimit
	}

	// CONVOY_SMTP_PROVIDER
	smtpProvider, err := cmd.Flags().GetString("smtp-provider")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(smtpProvider) {
		c.SMTP.Provider = smtpProvider
	}

	// CONVOY_SMTP_URL
	smtpUrl, err := cmd.Flags().GetString("smtp-url")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(smtpUrl) {
		c.SMTP.URL = smtpUrl
	}

	// CONVOY_SMTP_USERNAME
	smtpUsername, err := cmd.Flags().GetString("smtp-username")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(smtpUsername) {
		c.SMTP.Username = smtpUsername
	}

	// CONVOY_SMTP_PASSWORDvar configFile string
	smtpPassword, err := cmd.Flags().GetString("smtp-password")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(smtpPassword) {
		c.SMTP.Password = smtpPassword
	}

	// CONVOY_SMTP_FROM
	smtpFrom, err := cmd.Flags().GetString("smtp-from")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(smtpFrom) {
		c.SMTP.From = smtpFrom
	}

	// CONVOY_SMTP_REPLY_TO
	smtpReplyTo, err := cmd.Flags().GetString("smtp-reply-to")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(smtpReplyTo) {
		c.SMTP.ReplyTo = smtpReplyTo
	}

	// CONVOY_SMTP_PORT
	smtpPort, err := cmd.Flags().GetUint32("smtp-port")
	if err != nil {
		return err
	}

	if smtpPort != 0 {
		c.SMTP.Port = smtpPort
	}

	// CONVOY_MAX_RESPONSE_SIZE
	maxResponseSize, err := cmd.Flags().GetUint64("max-response-size")
	if err != nil {
		return err
	}

	if maxResponseSize != 0 {
		c.MaxResponseSize = maxResponseSize
	} else {
		c.MaxResponseSize = config.MaxResponseSizeKb
	}

	// CONVOY_NEWRELIC_APP_NAME
	newReplicApp, err := cmd.Flags().GetString("new-relic-app")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(newReplicApp) {
		c.NewRelic.AppName = newReplicApp
	}

	// CONVOY_NEWRELIC_LICENSE_KEY
	newReplicKey, err := cmd.Flags().GetString("new-relic-key")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(newReplicKey) {
		c.NewRelic.AppName = newReplicKey
	}

	// CONVOY_NEWRELIC_CONFIG_ENABLED
	isNRCESet := cmd.Flags().Changed("new-relic-config-enabled")
	if isNRCESet {
		newReplicConfigEnabled, err := cmd.Flags().GetBool("new-relic-config-enabled")
		if err != nil {
			return err
		}

		c.NewRelic.ConfigEnabled = newReplicConfigEnabled
	}

	// CONVOY_NEWRELIC_DISTRIBUTED_TRACER_ENABLED
	isNRTESet := cmd.Flags().Changed("new-relic-tracer-enabled")
	if isNRTESet {
		newReplicTracerEnabled, err := cmd.Flags().GetBool("new-relic-tracer-enabled")
		if err != nil {
			return err
		}

		c.NewRelic.DistributedTracerEnabled = newReplicTracerEnabled
	}

	// CONVOY_REQUIRE_AUTH
	isReqAuthSet := cmd.Flags().Changed("auth")
	if isReqAuthSet {
		requireAuth, err := cmd.Flags().GetBool("auth")
		if err != nil {
			return err
		}

		c.Auth.RequireAuth = requireAuth
	}

	// CONVOY_NATIVE_REALM_ENABLED
	isNativeRealmSet := cmd.Flags().Changed("native")
	if isNativeRealmSet {
		nativeRealmEnabled, err := cmd.Flags().GetBool("native")
		if err != nil {
			return err
		}

		c.Auth.Native.Enabled = nativeRealmEnabled
	}

	// CONVOY_API_KEY_CONFIG
	apiKeyAuthConfig, err := cmd.Flags().GetString("api-auth")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(apiKeyAuthConfig) {
		config := config.APIKeyAuthConfig{}
		err = config.Decode(apiKeyAuthConfig)
		if err != nil {
			return err
		}

		c.Auth.File.APIKey = config
	}

	// CONVOY_BASIC_AUTH_CONFIG
	basicAuthConfig, err := cmd.Flags().GetString("basic-auth")
	if err != nil {
		return err
	}

	if !util.IsStringEmpty(basicAuthConfig) {
		config := config.BasicAuthConfig{}
		err = config.Decode(basicAuthConfig)
		if err != nil {
			return err
		}

		c.Auth.File.Basic = config
	}

	return nil
}
