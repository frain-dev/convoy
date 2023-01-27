package main

import (
	"os"
	"os/signal"
	"time"

	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/spf13/cobra"
)

var (
	exit chan os.Signal
)

func addIngestCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Ingest webhook events from Pub/Sub streams",
		Run: func(cmd *cobra.Command, args []string) {
			sourceRepo := mongo.NewSourceRepo(a.store)
			endpointRepo := mongo.NewEndpointRepo(a.store)

			sourcePool := pubsub.NewSourcePool(a.queue, sourceRepo, endpointRepo)
			ticker := time.NewTicker(1 * time.Minute)
			exit = make(chan os.Signal)

			signal.Notify(exit, os.Interrupt)

			for {
				select {
				case <-ticker.C:
					page := 1
					err := sourcePool.FetchSources(page)

					if err != nil {
						log.WithError(err).Error("failed to fetch pub sub sources")
					}

				case <-exit:
					log.Println("Received SIGINT interrupt signal. Closing all pub sub sources")
					// Stop the ticker
					ticker.Stop()
					// Stop the existing pub sub sources
					sourcePool.Stop()
					return
				}
			}
		},
	}

	return cmd
}
