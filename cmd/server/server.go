package server

import (
	"errors"
	"time"

	"github.com/frain-dev/convoy/api"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/spf13/cobra"
)

func AddServerCommand(a *cli.App) *cobra.Command {
	var env string
	var host string
	var proxy string
	var sentry string
	var limiter string
	var cache string
	var logger string
	var searcher string
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
	var typesenseApiKey string
	var promaddr string

	var typesenseHost string
	var apiKeyAuthConfig string
	var basicAuthConfig string

	var ssl bool
	var disableEndpoint bool
	var replayAttacks bool
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
			// override config with cli flags
			cliConfig, err := buildServerCliConfiguration(cmd)
			if err != nil {
				return err
			}

			if err = config.Override(cliConfig); err != nil {
				return err
			}

			err = StartConvoyServer(a)

			if err != nil {
				a.Logger.Errorf("Error starting convoy server: %v", err)
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&apiKeyAuthConfig, "api-auth", "", "API-Key authentication credentials")
	cmd.Flags().StringVar(&basicAuthConfig, "basic-auth", "", "Basic authentication credentials")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "Log level")
	cmd.Flags().StringVar(&logger, "logger", "info", "Logger")
	cmd.Flags().StringVar(&proxy, "proxy", "", "HTTP Proxy")
	cmd.Flags().StringVar(&env, "env", "development", "Convoy environment")
	cmd.Flags().StringVar(&host, "host", "", "Host - The application host name")
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
	cmd.Flags().StringVar(&searcher, "searcher", "", "Searcher")
	cmd.Flags().StringVar(&typesenseHost, "typesense-host", "", "Typesense Host")
	cmd.Flags().StringVar(&typesenseApiKey, "typesense-api-key", "", "Typesense Api Key")
	cmd.Flags().StringVar(&promaddr, "promaddr", "", `Prometheus dsn`)

	cmd.Flags().BoolVar(&ssl, "ssl", false, "Configure SSL")
	cmd.Flags().BoolVar(&nativeRealmEnabled, "native", false, "Enable native-realm authentication")
	cmd.Flags().BoolVar(&disableEndpoint, "disable-endpoint", false, "Disable all application endpoints")
	cmd.Flags().BoolVar(&replayAttacks, "replay-attacks", false, "Enable feature to prevent replay attacks")
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

func StartConvoyServer(a *cli.App) error {
	cfg, err := config.Get()
	if err != nil {
		a.Logger.WithError(err).Fatal("Failed to load configuration")
	}

	start := time.Now()
	a.Logger.Info("Starting Convoy server...")

	apiKeyRepo := postgres.NewAPIKeyRepo(a.DB)
	userRepo := postgres.NewUserRepo(a.DB)
	err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, a.Cache)
	if err != nil {
		a.Logger.WithError(err).Fatal("failed to initialize realm chain")
	}

	if cfg.Server.HTTP.Port <= 0 {
		return errors.New("please provide the HTTP port in the convoy.json file")
	}

	lo := a.Logger.(*log.Logger)
	lo.SetPrefix("api server")

	lvl, err := log.ParseLevel(cfg.Logger.Level)
	if err != nil {
		return err
	}
	lo.SetLevel(lvl)

	srv := server.NewServer(cfg.Server.HTTP.Port, func() {})

	handler, err := api.NewApplicationHandler(
		&types.APIOptions{
			DB:     a.DB,
			Queue:  a.Queue,
			Logger: lo,
			Tracer: a.Tracer,
			Cache:  a.Cache,
		})
	if err != nil {
		return err
	}

	err = handler.RegisterPolicy()
	if err != nil {
		return err
	}

	srv.SetHandler(handler.BuildRoutes())

	a.Logger.Infof("Started convoy server in %s", time.Since(start))

	httpConfig := cfg.Server.HTTP
	if httpConfig.SSL {
		a.Logger.Infof("Started server with SSL: cert_file: %s, key_file: %s", httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		srv.ListenAndServeTLS(httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		return nil
	}

	a.Logger.Infof("Server running on port %v", cfg.Server.HTTP.Port)
	srv.Listen()
	return nil
}

func buildServerCliConfiguration(cmd *cobra.Command) (*config.Configuration, error) {
	c := &config.Configuration{}

	// CONVOY_ENV
	env, err := cmd.Flags().GetString("env")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(env) {
		c.Environment = env
	}

	// CONVOY_HOST
	host, err := cmd.Flags().GetString("host")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(host) {
		c.Host = host
	}

	// CONVOY_DB_TYPE
	dbType, err := cmd.Flags().GetString("db-type")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_SCHEME
	dbScheme, err := cmd.Flags().GetString("db-scheme")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_HOST
	dbHost, err := cmd.Flags().GetString("db-host")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_USERNAME
	dbUsername, err := cmd.Flags().GetString("db-username")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_PASSWORD
	dbPassword, err := cmd.Flags().GetString("db-password")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_DATABASE
	dbDatabase, err := cmd.Flags().GetString("db-database")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_PORT
	dbPort, err := cmd.Flags().GetInt("db-port")
	if err != nil {
		return nil, err
	}

	c.Database = config.DatabaseConfiguration{
		Type:     config.DatabaseProvider(dbType),
		Scheme:   dbScheme,
		Host:     dbHost,
		Username: dbUsername,
		Password: dbPassword,
		Database: dbDatabase,
		Port:     dbPort,
	}

	// CONVOY_REDIS_SCHEME
	redisScheme, err := cmd.Flags().GetString("redis-scheme")
	if err != nil {
		return nil, err
	}

	// CONVOY_REDIS_HOST
	redisHost, err := cmd.Flags().GetString("redis-host")
	if err != nil {
		return nil, err
	}

	// CONVOY_REDIS_USERNAME
	redisUsername, err := cmd.Flags().GetString("redis-username")
	if err != nil {
		return nil, err
	}

	// CONVOY_REDIS_PASSWORD
	redisPassword, err := cmd.Flags().GetString("redis-password")
	if err != nil {
		return nil, err
	}

	// CONVOY_REDIS_DATABASE
	redisDatabase, err := cmd.Flags().GetString("redis-database")
	if err != nil {
		return nil, err
	}

	// CONVOY_REDIS_PORT
	redisPort, err := cmd.Flags().GetInt("redis-port")
	if err != nil {
		return nil, err
	}

	c.Redis = config.RedisConfiguration{
		Scheme:   redisScheme,
		Host:     redisHost,
		Username: redisUsername,
		Password: redisPassword,
		Database: redisDatabase,
		Port:     redisPort,
	}

	// CONVOY_LOGGER_LEVEL
	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(logLevel) {
		c.Logger.Level = logLevel
	}

	// SSL
	isSslSet := cmd.Flags().Changed("ssl")
	if isSslSet {
		ssl, err := cmd.Flags().GetBool("ssl")
		if err != nil {
			return nil, err
		}

		c.Server.HTTP.SSL = ssl
	}

	// PORT
	port, err := cmd.Flags().GetUint32("port")
	if err != nil {
		return nil, err
	}

	if port != 0 {
		c.Server.HTTP.Port = port
	}

	// WORKER_PORT
	workerPort, err := cmd.Flags().GetUint32("worker-port")
	if err != nil {
		return nil, err
	}

	if workerPort != 0 {
		c.Server.HTTP.WorkerPort = workerPort
	}

	// CONVOY_SSL_KEY_FILE
	sslKeyFile, err := cmd.Flags().GetString("ssl-key-file")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(sslKeyFile) {
		c.Server.HTTP.SSLKeyFile = sslKeyFile
	}

	// CONVOY_SSL_CERT_FILE
	sslCertFile, err := cmd.Flags().GetString("ssl-cert-file")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(sslCertFile) {
		c.Server.HTTP.SSLCertFile = sslCertFile
	}

	// HTTP_PROXY
	proxy, err := cmd.Flags().GetString("proxy")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(proxy) {
		c.Server.HTTP.HttpProxy = proxy
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

	// CONVOY_MAX_RESPONSE_SIZE
	maxResponseSize, err := cmd.Flags().GetUint64("max-response-size")
	if err != nil {
		return nil, err
	}

	if maxResponseSize != 0 {
		c.MaxResponseSize = maxResponseSize
	}

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

	// CONVOY_SEARCH_TYPE
	searcher, err := cmd.Flags().GetString("searcher")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(searcher) {
		c.Search.Type = config.SearchProvider(searcher)
	}

	// CONVOY_TYPESENSE_HOST
	typesenseHost, err := cmd.Flags().GetString("typesense-host")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(typesenseHost) {
		c.Search.Typesense.Host = typesenseHost
	}

	// CONVOY_TYPESENSE_API_KEY
	typesenseApiKey, err := cmd.Flags().GetString("typesense-api-key")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(typesenseApiKey) {
		c.Search.Typesense.ApiKey = typesenseApiKey
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

	// CONVOY_NATIVE_REALM_ENABLED
	isNativeRealmSet := cmd.Flags().Changed("native")
	if isNativeRealmSet {
		nativeRealmEnabled, err := cmd.Flags().GetBool("native")
		if err != nil {
			return nil, err
		}

		c.Auth.Native.Enabled = nativeRealmEnabled
	}

	// CONVOY_API_KEY_CONFIG
	apiKeyAuthConfig, err := cmd.Flags().GetString("api-auth")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(apiKeyAuthConfig) {
		authConfig := config.APIKeyAuthConfig{}
		err = authConfig.Decode(apiKeyAuthConfig)
		if err != nil {
			return nil, err
		}

		c.Auth.File.APIKey = authConfig
	}

	// CONVOY_BASIC_AUTH_CONFIG
	basicAuthConfig, err := cmd.Flags().GetString("basic-auth")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(basicAuthConfig) {
		authConfig := config.BasicAuthConfig{}
		err = authConfig.Decode(basicAuthConfig)
		if err != nil {
			return nil, err
		}

		c.Auth.File.Basic = authConfig
	}

	return c, nil
}
