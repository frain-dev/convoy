package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/server"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addWorkerCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start worker instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			// register tasks.
			handler := task.ProcessEventDelivery(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo)
			if err := task.CreateTasks(a.groupRepo, handler); err != nil {
				log.WithError(err).Error("failed to register tasks")
				return err
			}

			worker.RegisterNewGroupTask(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo)
			// register workers.
			ctx := context.Background()
			producer := worker.NewProducer(a.eventQueue)
			if cfg.Queue.Type != config.InMemoryQueueProvider {
				producer.Start(ctx)
			}
			worker.RegisterWorkerMetrics(a.eventQueue, cfg)
			server.RegisterQueueMetrics(a.eventQueue, cfg)

			router := chi.NewRouter()
			router.Handle("/v1/metrics", promhttp.Handler())
			router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				render.JSON(w, r, "Convoy")
			})

			srv := &http.Server{
				Handler: router,
				Addr:    fmt.Sprintf(":%d", cfg.Server.HTTP.WorkerPort),
			}

			log.Infof("Worker running on port %v", cfg.Server.HTTP.WorkerPort)

			e := srv.ListenAndServe()
			if e != nil {
				return e
			}

			<-ctx.Done()
			return ctx.Err()
		},
	}
	return cmd
}
