package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/analytics"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"

	"github.com/frain-dev/convoy/server"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
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
			//Initialize queue
			rdb, err := rdb.NewClient(cfg.Queue.Redis.Dsn)
			if err != nil {
				log.WithError(err).Fatalf("Unable to create redis client")
			}
			queueNames := map[string]int{
				string(convoy.DefaultQueue): 10,
			}
			opts := queue.QueueOptions{
				Names:             queueNames,
				RedisClient:       rdb,
				RedisAddress:      cfg.Queue.Redis.Dsn,
				Type:              string(config.RedisQueueProvider),
				PrometheusAddress: cfg.Prometheus.Dsn,
			}
			q := redisqueue.NewQueue(opts)
			ctx := context.Background()

			//initialize consumer
			w, err := worker.NewConsumer(q)
			if err != nil {
				log.WithError(err).Fatal("error creating consumer")
			}
			//initialize scheduler
			s := worker.NewScheduler(q)

			//register tasks
			w.RegisterHandlers(convoy.TaskName("daily analytics"), analytics.TrackDailyAnalytics(&analytics.Repo{
				ConfigRepo: a.configRepo,
				EventRepo:  a.eventRepo,
				GroupRepo:  a.groupRepo,
				OrgRepo:    a.orgRepo,
				UserRepo:   a.userRepo,
			}, cfg))
			w.RegisterHandlers(convoy.TaskName("monitor twitter sources"), task.MonitorTwitterSources(a.sourceRepo, a.subRepo, a.applicationRepo, a.queue))
			s.RegisterTask("55 23 * * *", convoy.DefaultQueue, convoy.TaskName("daily analytics"))
			s.RegisterTask("30 * * * *", convoy.DefaultQueue, convoy.TaskName("monitor twitter sources"))

			// Start scheduler
			s.Start()

			//start worker
			w.Start()

			router := chi.NewRouter()
			router.Handle("/queue/monitoring/*", q.(*redisqueue.RedisQueue).Monitor())
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
