package dataplane

import (
	"fmt"
	"time"

	"github.com/frain-dev/convoy/api"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/api_keys"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/internal/portal_links"
	"github.com/frain-dev/convoy/internal/users"
)

func StartServer(opts RuntimeOpts, cfg config.Configuration) error {
	lo := opts.Logger

	start := time.Now()
	lo.Info("Starting Convoy data plane")

	userRepo := users.New(opts.Logger, opts.DB)
	apiKeyRepo := api_keys.New(opts.Logger, opts.DB)
	configRepo := configuration.New(opts.Logger, opts.DB)
	portalLinkRepo := portal_links.New(opts.Logger, opts.DB)
	err := realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, opts.Cache, opts.Logger)
	if err != nil {
		return fmt.Errorf("failed to initialize realm chain: %w", err)
	}

	flag := fflag.NewFFlag(cfg.EnableFeatureFlag)
	srv := server.NewServer(cfg.Server.HTTP.AgentPort, func() {})

	evHandler, err := api.NewApplicationHandler(
		&types.APIOptions{
			FFlag:      flag,
			DB:         opts.DB,
			Queue:      opts.Queue,
			Logger:     lo,
			Cache:      opts.Cache,
			Rate:       opts.Rate,
			Redis:      opts.Redis,
			Licenser:   opts.Licenser,
			Cfg:        cfg,
			ConfigRepo: configRepo,
		})
	if err != nil {
		return fmt.Errorf("failed to create application handler: %w", err)
	}

	srv.SetHandler(evHandler.BuildDataPlaneRoutes())

	lo.Infof("Started convoy server in %s", time.Since(start))

	httpConfig := cfg.Server.HTTP
	if httpConfig.SSL {
		opts.Logger.Infof("Started server with SSL: cert_file: %s, key_file: %s", httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		srv.ListenAndServeTLS(httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		return nil
	}

	lo.Infof("Starting Convoy Agent on port %v", cfg.Server.HTTP.AgentPort)

	go func() {
		srv.Listen()
	}()

	return nil
}
