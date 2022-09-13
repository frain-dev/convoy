package main

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func addMigrateCommand(app *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "migrate the db schema from the rc build schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}

			db, err := cm.New(cfg)
			if err != nil {
				return err
			}

			type RTConfig struct {
				Duration string `json:"duration"`
			}

			type Config struct {
				RateLimit RTConfig `json:"ratelimit"`
			}

			type Group struct {
				UID            string                   `json:"uid" bson:"uid"`
				Config         Config                   `json:"config" bson:"config"`
				DocumentStatus datastore.DocumentStatus `json:"-" bson:"document_status"`
			}

			store := datastore.New(db.Client().(*mongo.Database), cm.GroupCollection)

			var groups []*Group
			err = store.FindAll(context.Background(), nil, nil, nil, &groups)
			if err != nil {
				return err
			}

			var newDuration uint64
			for _, group := range groups {
				duration, err := time.ParseDuration(group.Config.RateLimit.Duration)
				if err != nil {
					// Set default when an error occurs.
					newDuration = datastore.DefaultRateLimitConfig.Duration
				} else {
					newDuration = uint64(duration.Seconds())
				}

				update := bson.M{"config.ratelimit.duration": newDuration}
				err = store.UpdateByID(context.Background(), group.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration")
					return err
				}
			}

			return nil
		},
	}

	return cmd
}
