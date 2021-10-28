package main

import (
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	convoyQueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/server"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker"
	convoyTask "github.com/frain-dev/convoy/worker/task"
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

			srv := server.New(cfg, a.eventRepo, a.applicationRepo, a.groupRepo, a.eventQueue, a.eventDeliveryQueue)

			// register workers.
			if queue, ok := a.eventQueue.(*convoyQueue.RedisQueue); ok {
				worker.NewProducer(queue).Start()
			}

			if queue, ok := a.eventDeliveryQueue.(*convoyQueue.RedisQueue); ok {
				worker.NewProducer(queue).Start()
			}

			if queue, ok := a.deadLetterQueue.(*convoyQueue.RedisQueue); ok {
				worker.NewCleaner(queue).Start()
			}

			// register tasks.
			convoyTask.CreateTask(convoy.EventProcessor, cfg, convoyTask.ProcessEvent(a.eventRepo, a.eventDeliveryQueue))
			convoyTask.CreateTask(convoy.EventDeliveryProcessor, cfg, convoyTask.ProcessEventDeliveries(a.applicationRepo, a.eventRepo, a.groupRepo))
			convoyTask.CreateTask(convoy.DeadLetterProcessor, cfg, convoyTask.ProcessDeadLetters)

			log.Infof("Started convoy server in %s", time.Since(start))
			return srv.ListenAndServe()
		},
	}

	return cmd
}
