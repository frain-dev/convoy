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

	cmd.Flags().BoolVarP(&withWorkers, "with-workers", "w", true, "Should run workers")

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

	srv := server.New(cfg, a.eventRepo, a.eventDeliveryRepo, a.applicationRepo, a.apiKeyRepo, a.groupRepo, a.eventQueue, a.logger, a.tracer, a.cache)

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
	return srv.ListenAndServe()
}
