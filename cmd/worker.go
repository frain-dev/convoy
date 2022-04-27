package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/server"
	"github.com/frain-dev/convoy/worker"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addWorkerCommand(a *app) *cobra.Command {
	var workerPort uint32

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start worker instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}

			worker.RegisterNewGroupTask(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo, a.limiter, a.eventRepo, a.cache, a.eventQueue, a.tracer)
			// register workers.
			ctx := context.Background()
			eventCreationProducer := worker.NewProducer(a.createEventQueue)
			if cfg.Queue.Type != config.InMemoryQueueProvider {
				eventCreationProducer.Start(ctx)
			}

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
				Addr:    fmt.Sprintf(":%d", workerPort),
			}

			log.Infof("Worker running on port %v", workerPort)

			e := srv.ListenAndServe()
			if e != nil {
				return e
			}

			<-ctx.Done()
			return ctx.Err()
		},
	}

	cmd.Flags().Uint32Var(&workerPort, "worker-port", 5006, "Worker port")
	return cmd
}
