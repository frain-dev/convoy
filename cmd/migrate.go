package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/migrate"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func addMigrateCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convoy migrations",
	}

	cmd.AddCommand(addUpCommand())
	cmd.AddCommand(addDownCommand())

	return cmd
}

func addUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Run all pending migrations",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Fatalf("Error fetching the config.")
			}

			db, err := cm.New(cfg)
			if err != nil {
				log.WithError(err).Fatalf("Error instantiating a database client")
			}

			c := db.Client().(*mongo.Database).Client()

			u, err := url.Parse(cfg.Database.Dsn)
			if err != nil {
				log.WithError(err).Error("Error parsing database url")
			}

			dbName := strings.TrimPrefix(u.Path, "/")
			opts := &migrate.Options{
				DatabaseName: dbName,
			}

			m := migrate.NewMigrator(c, opts, migrations, nil)

			err = m.Migrate(context.Background())
			if err != nil {
				log.WithError(err).Fatalf("Error running migrations")
			}
		},
	}

	return cmd
}

func addDownCommand() *cobra.Command {
	var migrationID string

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Rollback migrations",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Fatalf("Error fetching the config.")
			}

			db, err := cm.New(cfg)
			if err != nil {
				log.WithError(err).Fatalf("Error instantiating a database client")
			}

			c := db.Client().(*mongo.Database).Client()

			u, err := url.Parse(cfg.Database.Dsn)
			if err != nil {
				log.WithError(err).Error("Error parsing database url")
			}

			dbName := strings.TrimPrefix(u.Path, "/")
			opts := &migrate.Options{
				DatabaseName: dbName,
			}

			m := migrate.NewMigrator(c, opts, migrations, nil)

			err = m.RollbackTo(context.Background(), migrationID)
			if err != nil {
				log.WithError(err).Fatalf("Error rolling back migrations")
			}
		},
	}

	cmd.Flags().StringVar(&migrationID, "id", "", "Migration ID")

	return cmd
}

// MIGRATIONS

var (
	migrations = []*migrate.Migration{
		{
			ID: "20220901162904_change_group_rate_limit_configuration",
			Migrate: func(db *mongo.Database) error {
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

				store := datastore.New(db, cm.GroupCollection)

				var groups []*Group
				err := store.FindAll(context.Background(), nil, nil, nil, &groups)
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
			Rollback: func(db *mongo.Database) error {

				store := datastore.New(db, cm.GroupCollection)

				var groups []*datastore.Group
				err := store.FindAll(context.Background(), nil, nil, nil, &groups)
				if err != nil {
					return err
				}

				var newDuration time.Duration
				for _, group := range groups {
					duration := fmt.Sprintf("%ds", group.Config.RateLimit.Duration)
					newDuration, err = time.ParseDuration(duration)
					if err != nil {
						return err
					}

					update := bson.M{"config.ratelimit.duration": newDuration.String()}
					err = store.UpdateByID(context.Background(), group.UID, update)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration")
						return err
					}
				}

				return nil
			},
		},
	}
)
