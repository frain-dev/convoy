package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/pkg/log"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/worker"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

func addSchedulerCommand(a *app) *cobra.Command {
	var exportCronSpec string
	var port uint32
	var logLevel string
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "scheduler runs periodic tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				a.logger.Errorf("Failed to retrieve config: %v", err)
				return err
			}

			lo := a.logger.(*log.Logger)
			lo.SetPrefix("scheduler")

			lvl, err := log.ParseLevel(cfg.Logger.Level)
			if err != nil {
				return err
			}
			lo.SetLevel(lvl)

			ctx := context.Background()

			//initialize scheduler
			s := worker.NewScheduler(a.queue, lo)

			//register tasks
			s.RegisterTask("30 * * * *", convoy.ScheduleQueue, convoy.MonitorTwitterSources)
			s.RegisterTask("55 23 * * *", convoy.ScheduleQueue, convoy.DailyAnalytics)
			s.RegisterTask(exportCronSpec, convoy.ScheduleQueue, convoy.RetentionPolicies)

			// Start scheduler
			s.Start()

			router := chi.NewRouter()
			router.Handle("/queue/monitoring/*", a.queue.(*redisqueue.RedisQueue).Monitor())
			router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))

			srv := &http.Server{
				Handler: router,
				Addr:    fmt.Sprintf(":%d", port),
			}

			e := srv.ListenAndServe()
			if e != nil {
				a.logger.Fatalf("scheduler crashed: %v", e)
			}
			<-ctx.Done()

			return nil
		},
	}

	cmd.Flags().StringVar(&exportCronSpec, "export-spec", "@every 24h", "export scheduler time interval '@every <duration>'")
	cmd.Flags().Uint32Var(&port, "port", 5007, "port to serve metrics")
	cmd.Flags().StringVar(&logLevel, "log-level", "error", "scheduler log level")
	return cmd
}
