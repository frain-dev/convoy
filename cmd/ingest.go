package main

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/spf13/cobra"
)

var (
	sourcePool = pubsub.NewSourcePool()
)

func addIngestCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Ingest webhook events from Pub/Sub streams",
		Run: func(cmd *cobra.Command, args []string) {
			ticker := time.NewTicker(1 * time.Minute)

			for {
				select {
				case <-ticker.C:
					sourceRepo := mongo.NewSourceRepo(a.store)
					err := fetchPubSubSources(sourceRepo, 1)
					if err != nil {
						log.WithError(err).Error("failed to fetch pub sub sources")
					}
				}
			}
		},
	}

	return cmd
}

func fetchPubSubSources(sourceRepo datastore.SourceRepository, page int) error {
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
	return fetchPubSubSources(sourceRepo, page)
}
