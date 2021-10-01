package main

import (
	"errors"
	"time"

	"github.com/frain-dev/convoy/config"
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

			if util.IsStringEmpty(string(cfg.Signature.Header)) {
				cfg.Signature.Header = config.DefaultSignatureHeader
				log.Warnf("signature header is blank. setting default %s", config.DefaultSignatureHeader)
			}

			if cfg.Server.HTTP.Port <= 0 {
				return errors.New("please provide the HTTP port in the convoy.json file")
			}

			srv := server.New(cfg, a.messageRepo, a.applicationRepo, a.orgRepo)

			worker.NewCleaner(&a.queue, &a.messageRepo).Start()
			worker.NewScheduler(&a.queue, &a.messageRepo).Start()
			worker.NewProducer(&a.queue, &a.applicationRepo, &a.messageRepo, cfg.Signature, cfg.SMTP).Start()

			log.Infof("Started convoy server in %s", time.Since(start))
			return srv.ListenAndServe()
		},
	}

	return cmd
}
