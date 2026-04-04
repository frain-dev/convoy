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
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/internal/portal_links"
	"github.com/frain-dev/convoy/internal/users"
)

func StartServer(a *cli.App, cfg config.Configuration) error {
	lo := a.Logger

	start := time.Now()
	lo.Info("Starting Convoy data plane")

	userRepo := users.New(a.Logger, a.DB)
	apiKeyRepo := api_keys.New(a.Logger, a.DB)
	configRepo := configuration.New(a.Logger, a.DB)
	portalLinkRepo := portal_links.New(a.Logger, a.DB)
	err := realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, a.Cache, a.Logger)
	if err != nil {
		return fmt.Errorf("failed to initialize realm chain: %w", err)
	}

	flag := fflag.NewFFlag(cfg.EnableFeatureFlag)
	srv := server.NewServer(cfg.Server.HTTP.AgentPort, func() {})

	evHandler, err := api.NewApplicationHandler(
		&types.APIOptions{
			FFlag:      flag,
			DB:         a.DB,
			Queue:      a.Queue,
			Logger:     lo,
			Cache:      a.Cache,
			Rate:       a.Rate,
			Redis:      a.Redis,
			Licenser:   a.Licenser,
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
