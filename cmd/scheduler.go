package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	redisqueue "github.com/frain-dev/convoy/queue/redis"

	"github.com/frain-dev/convoy/server"
	"github.com/frain-dev/convoy/worker"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addSchedulerCommand(a *app) *cobra.Command {
	var cronspec string
	var port uint32
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "schedule a periodic task.",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				log.Fatalf("Error getting config: %v", err)
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.WithError(err).Fatalf("Queue type error: Command is available for redis queue only.")
			}
			ctx := context.Background()

			//initialize scheduler
			s := worker.NewScheduler(a.queue)

			//register tasks
			s.RegisterTask("30 * * * *", convoy.ScheduleQueue, convoy.MonitorTwitterSources)
			s.RegisterTask("55 23 * * *", convoy.ScheduleQueue, convoy.DailyAnalytics)
			s.RegisterTask("@every 24h", convoy.ScheduleQueue, convoy.RetentionPolicies)

			// Start scheduler
			s.Start()

			router := chi.NewRouter()
			router.Handle("/queue/monitoring/*", a.queue.(*redisqueue.RedisQueue).Monitor())
			router.Handle("/metrics", promhttp.HandlerFor(server.Reg, promhttp.HandlerOpts{}))

			srv := &http.Server{
				Handler: router,
				Addr:    fmt.Sprintf(":%d", port),
			}

			e := srv.ListenAndServe()
			if e != nil {
				log.Fatal(e)
			}
			<-ctx.Done()
		},
	}

	cmd.Flags().StringVar(&cronspec, "cronspec", "", "scheduler time interval '@every <duration>'")
	cmd.Flags().Uint32Var(&port, "port", 5007, "port to serve Metrics")
	return cmd
}
