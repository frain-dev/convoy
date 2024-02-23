package server

import (
	"errors"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/worker"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/ee/api"
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
	var promaddr string

	var apiKeyAuthConfig string
	var basicAuthConfig string

	var ssl bool
	var disableEndpoint bool
	var replayAttacks bool
	var multipleTenants bool
	var nativeRealmEnabled bool

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
	cmd.Flags().StringVar(&logLevel, "log-level", "error", "Log level")
	cmd.Flags().StringVar(&logger, "logger", "info", "Logger")
	cmd.Flags().StringVar(&proxy, "proxy", "", "HTTP Proxy")
	cmd.Flags().StringVar(&env, "env", "development", "Convoy environment")
	cmd.Flags().StringVar(&host, "host", "", "Host - The application host name")
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
	cmd.Flags().StringVar(&promaddr, "promaddr", "", `Prometheus dsn`)

	cmd.Flags().BoolVar(&ssl, "ssl", false, "Configure SSL")
	cmd.Flags().BoolVar(&nativeRealmEnabled, "native", false, "Enable native-realm authentication")
	cmd.Flags().BoolVar(&disableEndpoint, "disable-endpoint", false, "Disable all application endpoints")
	cmd.Flags().BoolVar(&replayAttacks, "replay-attacks", false, "Enable feature to prevent replay attacks")
	cmd.Flags().BoolVar(&multipleTenants, "multi-tenant", false, "Start convoy in single- or multi-tenant mode")

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

	handler, err := api.NewEHandler(
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

	err = handler.RegisterPolicy()
	if err != nil {
		return err
	}

	srv.SetHandler(handler.BuildRoutes())

	// initialize scheduler
	s := worker.NewScheduler(a.Queue, lo)

	// register daily analytic task
	s.RegisterTask("58 23 * * *", convoy.ScheduleQueue, convoy.DeleteArchivedTasksProcessor)
	s.RegisterTask("30 * * * *", convoy.ScheduleQueue, convoy.MonitorTwitterSources)
	s.RegisterTask("0 0 * * *", convoy.ScheduleQueue, convoy.RetentionPolicies)
	s.RegisterTask("55 23 * * *", convoy.ScheduleQueue, convoy.DailyAnalytics)
	s.RegisterTask("0 * * * *", convoy.ScheduleQueue, convoy.TokenizeSearch)

	// Start scheduler
	s.Start()

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
