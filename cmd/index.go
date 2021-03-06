package main

import (
	"context"

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
			coll := db.Collection("events")
			ctx := context.Background()

			cs, err := coll.Watch(ctx, mongo.Pipeline{})
			if err != nil {
				return err
			}
			defer cs.Close(ctx)

			for {
				ok := cs.Next(ctx)
				if ok {
					var document *convoy.GenericMap
					err := cs.Decode(&document)
					if err != nil {
						return err
					}

					if (*document)["operationType"].(string) == "insert" {
						doc := (*document)["fullDocument"].(convoy.GenericMap)
						err := a.searcher.Index("events", doc)
						if err != nil {
							return err
						}
					}
				}
			}
		},
	}

	return cmd
}
