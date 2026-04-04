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

func StartServer(deps RuntimeDeps, cfg config.Configuration) error {
	lo := deps.Logger

	start := time.Now()
	lo.Info("Starting Convoy data plane")

	userRepo := users.New(deps.Logger, deps.DB)
	apiKeyRepo := api_keys.New(deps.Logger, deps.DB)
	configRepo := configuration.New(deps.Logger, deps.DB)
	portalLinkRepo := portal_links.New(deps.Logger, deps.DB)
	err := realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, deps.Cache, deps.Logger)
	if err != nil {
		return fmt.Errorf("failed to initialize realm chain: %w", err)
	}

	flag := fflag.NewFFlag(cfg.EnableFeatureFlag)
	srv := server.NewServer(cfg.Server.HTTP.AgentPort, func() {})

	evHandler, err := api.NewApplicationHandler(
		&types.APIOptions{
			FFlag:      flag,
			DB:         deps.DB,
			Queue:      deps.Queue,
			Logger:     lo,
			Cache:      deps.Cache,
			Rate:       deps.Rate,
			Redis:      deps.Redis,
			Licenser:   deps.Licenser,
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
		deps.Logger.Infof("Started server with SSL: cert_file: %s, key_file: %s", httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		srv.ListenAndServeTLS(httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		return nil
	}

	lo.Infof("Starting Convoy Agent on port %v", cfg.Server.HTTP.AgentPort)

	go func() {
		srv.Listen()
	}()

	return nil
}
