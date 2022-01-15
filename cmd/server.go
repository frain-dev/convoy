package main

import (
	"errors"
	"time"

	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/worker/task"

	"github.com/frain-dev/convoy/config"
	convoyQueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/server"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addServerCommand(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "server",
		Aliases: []string{"serve", "s"},
		Short:   "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			log.Info("Starting Convoy server...")

			cfg, err := config.Get()
			if err != nil {
				return err
			}

			if util.IsStringEmpty(string(cfg.GroupConfig.Signature.Header)) {
				cfg.GroupConfig.Signature.Header = config.DefaultSignatureHeader
				log.Warnf("signature header is blank. setting default %s", config.DefaultSignatureHeader)
			}

			err = realm_chain.Init(&cfg.Auth, a.apiKeyRepo)
			if err != nil {
				log.WithError(err).Fatal("failed to initialize realm chain")
			}

			if cfg.Server.HTTP.Port <= 0 {
				return errors.New("please provide the HTTP port in the convoy.json file")
			}

			srv := server.New(cfg, a.eventRepo, a.eventDeliveryRepo, a.applicationRepo, a.apiKeyRepo, a.groupRepo, a.eventQueue)

			// register tasks.
			handler := task.ProcessEventDelivery(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo)
			if err := task.CreateTasks(a.groupRepo, handler); err != nil {
				log.WithError(err).Error("failed to register tasks")
				return err
			}

			// register workers.
			if queue, ok := a.eventQueue.(*convoyQueue.RedisQueue); ok {
				worker.NewProducer(queue).Start()
			}

			if queue, ok := a.deadLetterQueue.(*convoyQueue.RedisQueue); ok {
				worker.NewCleaner(queue).Start()
			}

			log.Infof("Started convoy server in %s", time.Since(start))

			httpConfig := cfg.Server.HTTP
			if httpConfig.SSL {
				log.Infof("Started server with SSL: cert_file: %s, key_file: %s", httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
				return srv.ListenAndServeTLS(httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
			}
			return srv.ListenAndServe()
		},
	}

	return cmd
}
