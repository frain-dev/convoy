package main

import (
	"errors"
	"github.com/hookcamp/hookcamp/config"
	"github.com/hookcamp/hookcamp/server"
	"github.com/hookcamp/hookcamp/worker"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"time"
)

func addServerCommand(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "server",
		Aliases: []string{"serve", "s"},
		Short:   "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			log.Info("Starting Hookcamp server...")

			cfg, err := config.Get()
			if err != nil {
				return err
			}

			if cfg.Server.HTTP.Port <= 0 {
				return errors.New("please provide the HTTP port in the hookcamp.json file")
			}

			srv := server.New(cfg, a.messageRepo, a.applicationRepo, a.orgRepo)

			worker.NewCleaner(&a.queue, &a.messageRepo).Start()
			worker.NewScheduler(&a.queue, &a.messageRepo).Start()
			worker.NewProducer(&a.queue, &a.messageRepo, cfg.Signature.Header).Start()

			log.Infof("Started Hookcamp server in %s", time.Since(start))
			return srv.ListenAndServe()
		},
	}

	return cmd
}
