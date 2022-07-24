package main

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/frain-dev/convoy/searcher"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	m "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
)

func addIndexCommand(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Starts events search indexer",
		RunE: func(cmd *cobra.Command, args []string) error {
			indexFn := indexNewDocuments(m.EventCollection, a.searcher)
			err := watchCollection(indexFn, mongo.Pipeline{}, m.EventCollection, nil)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func indexNewDocuments(collection string, searcher searcher.Searcher) WatcherFn {
	return func(doc convoy.GenericMap) error {
		return searcher.Index(collection, doc)
	}
}

type WatcherFn func(convoy.GenericMap) error

func watchCollection(fn func(convoy.GenericMap) error, pipeline mongo.Pipeline, collection string, stop chan struct{}) error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}

	if cfg.Database.Type != "mongodb" {
		return convoy.ErrUnsupportedDatebase
	}

	client, err := m.New(cfg)
	if err != nil {
		return err
	}

	db := client.Client().(*mongo.Database)
	coll := db.Collection(collection)
	ctx := context.Background()

	cs, err := coll.Watch(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cs.Close(ctx)

	for {
		select {
		case <-stop:
			logrus.Println("Exiting Database watcher")
			return nil
		default:
			ok := cs.Next(ctx)
			if ok {
				var document *convoy.GenericMap
				err := cs.Decode(&document)
				if err != nil {
					return err
				}

				if (*document)["operationType"].(string) == "insert" {
					doc := (*document)["fullDocument"].(convoy.GenericMap)
					err := fn(doc)
					if err != nil {
						return err
					}
				}
			}
		}
	}
}
