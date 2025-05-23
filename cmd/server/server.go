package server

import (
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	_ "net/http/pprof"
	"strings"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/fflag"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/worker"

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
	var limiter string
	var cache string
	var logger string
	var sslKeyFile string
	var sslCertFile string
	var promaddr string

	var apiKeyAuthConfig string
	var basicAuthConfig string
	var nativeRealmEnabled bool

	var ssl bool
	var port uint32
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

			err = startConvoyServer(a)

			if err != nil {
				a.Logger.Errorf("Error starting convoy server: %v", err)
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&apiKeyAuthConfig, "api-auth", "", "API-Key authentication credentials")
	cmd.Flags().StringVar(&basicAuthConfig, "basic-auth", "", "Basic authentication credentials")
	cmd.Flags().StringVar(&logger, "logger", "info", "Logger")
	cmd.Flags().StringVar(&proxy, "proxy", "", "HTTP Proxy")
	cmd.Flags().StringVar(&env, "env", "development", "Convoy environment")
	cmd.Flags().StringVar(&host, "host", "", "Host - The application host name")
	cmd.Flags().StringVar(&cache, "cache", "redis", `Cache Provider ("redis" or "in-memory")`)
	cmd.Flags().StringVar(&limiter, "limiter", "redis", `Rate limiter provider ("redis" or "in-memory")`)
	cmd.Flags().StringVar(&sslCertFile, "ssl-cert-file", "", "SSL certificate file")
	cmd.Flags().StringVar(&sslKeyFile, "ssl-key-file", "", "SSL key file")
	cmd.Flags().StringVar(&promaddr, "promaddr", "", `Prometheus dsn`)

	cmd.Flags().BoolVar(&ssl, "ssl", false, "Configure SSL")
	cmd.Flags().BoolVar(&nativeRealmEnabled, "native", false, "Enable native-realm authentication")

	cmd.Flags().Uint32Var(&port, "port", 0, "Server port")
	cmd.Flags().Uint64Var(&maxResponseSize, "max-response-size", 0, "Max response size")

	return cmd
}

func startConvoyServer(a *cli.App) error {
	cfg, err := config.Get()
	if err != nil {
		a.Logger.WithError(err).Fatal("Failed to load configuration")
	}

	start := time.Now()
	a.Logger.Info("Starting Convoy control plane...")

	km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, a.Licenser, a.Cache)
	if km.IsSet() {
		if _, err = km.GetCurrentKeyFromCache(); err != nil {
			if !errors.Is(err, keys.ErrCredentialEncryptionFeatureUnavailable) {
				return err
			}
			km.Unset()
		}
	}
	if err = keys.Set(km); err != nil {
		return err
	}

	apiKeyRepo := postgres.NewAPIKeyRepo(a.DB)
	userRepo := postgres.NewUserRepo(a.DB)
	portalLinkRepo := postgres.NewPortalLinkRepo(a.DB)
	err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, a.Cache)
	if err != nil {
		a.Logger.WithError(err).Fatal("failed to initialize realm chain")
	}

	flag := fflag.NewFFlag(cfg.EnableFeatureFlag)

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
			FFlag:    flag,
			DB:       a.DB,
			Queue:    a.Queue,
			Logger:   lo,
			Redis:    a.Redis,
			Cache:    a.Cache,
			Rate:     a.Rate,
			Licenser: a.Licenser,
			Cfg:      cfg,
		})
	if err != nil {
		return err
	}

	err = handler.RegisterPolicy()
	if err != nil {
		return err
	}

	srv.SetHandler(handler.BuildControlPlaneRoutes())

	// initialize scheduler
	s := worker.NewScheduler(a.Queue, lo)

	// register tasks
	s.RegisterTask("58 23 * * *", convoy.ScheduleQueue, convoy.DeleteArchivedTasksProcessor)
	s.RegisterTask("30 * * * *", convoy.ScheduleQueue, convoy.MonitorTwitterSources)
	s.RegisterTask("0 * * * *", convoy.ScheduleQueue, convoy.TokenizeSearch)

	// ensures that project data is backed up about 2 hours before they are deleted
	if a.Licenser.RetentionPolicy() {
		// runs at 10pm
		s.RegisterTask("0 22 * * *", convoy.ScheduleQueue, convoy.BackupProjectData)
		// runs at 1am
		s.RegisterTask("0 1 * * *", convoy.ScheduleQueue, convoy.RetentionPolicies)
	}

	metrics.RegisterQueueMetrics(a.Queue, a.DB, nil)

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

	replicaDSNs, err := cmd.Flags().GetStringSlice("read-replicas-dsn")
	if err != nil {
		return nil, err
	}

	var readReplicas []config.DatabaseConfiguration
	for _, replicaStr := range replicaDSNs {
		var replica config.DatabaseConfiguration
		if len(replicaStr) == 0 || !strings.Contains(replicaStr, "://") {
			return nil, fmt.Errorf("invalid read-replicas-dsn: %s", replicaStr)
		}
		replica.DSN = replicaStr
		readReplicas = append(readReplicas, replica)
	}

	c.Database = config.DatabaseConfiguration{
		Type:     config.DatabaseProvider(dbType),
		Scheme:   dbScheme,
		Host:     dbHost,
		Username: dbUsername,
		Password: dbPassword,
		Database: dbDatabase,
		Port:     dbPort,

		ReadReplicas: readReplicas,
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
