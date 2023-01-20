package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/frain-dev/convoy/datastore"
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
			sourcePool := pubsub.NewSourcePool(a.queue)
			ticker := time.NewTicker(1 * time.Minute)
			exit = make(chan os.Signal)

			signal.Notify(exit, os.Interrupt)

			for {
				select {
				case <-ticker.C:
					sourceRepo := mongo.NewSourceRepo(a.store)
					err := fetchPubSubSources(sourceRepo, sourcePool, 1)
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

func fetchPubSubSources(sourceRepo datastore.SourceRepository, sourcePool *pubsub.SourcePool, page int) error {
	filter := &datastore.SourceFilter{
		Type: string(datastore.PubSubSource),
	}

	pageable := datastore.Pageable{
		Page:    page,
		PerPage: 50,
	}

	sources, _, err := sourceRepo.LoadSourcesPaged(context.Background(), "", filter, pageable)
	if err != nil {
		return err
	}

	if len(sources) == 0 {
		return nil
	}

	for _, source := range sources {
		err = sourcePool.Insert(&source)
		if err != nil {
			log.WithError(err).Error("failed to insert pub sub sources")
			continue
		}
	}

	page += 1
	return fetchPubSubSources(sourceRepo, sourcePool, page)
}
