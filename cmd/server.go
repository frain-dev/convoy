package main

import (
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	convoy_queue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/server"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker"
	convoy_task "github.com/frain-dev/convoy/worker/task"
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

			srv := server.New(cfg, a.messageRepo, a.applicationRepo, a.groupRepo, a.scheduleQueue)

			// register workers.
			if queue, ok := a.scheduleQueue.(*convoy_queue.RedisQueue); ok {
				worker.NewProducer(queue).Start()
			}

			if queue, ok := a.deadLetterQueue.(*convoy_queue.RedisQueue); ok {
				worker.NewCleaner(queue).Start()
			}

			// register tasks.
			convoy_task.CreateTask(convoy.EventProcessor, cfg, convoy_task.ProcessMessages(a.applicationRepo, a.messageRepo, a.groupRepo))
			convoy_task.CreateTask(convoy.DeadLetterProcessor, cfg, convoy_task.ProcessDeadLetters)

			log.Infof("Started convoy server in %s", time.Since(start))

			httpConfig := cfg.Server.HTTP
			if httpConfig.SSl {
				log.Infof("Started server with SSL: cert_file: %s, key_file: %s", httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
				return srv.ListenAndServeTLS(httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
			}
			return srv.ListenAndServe()
		},
	}

	return cmd
}
