package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/frain-dev/convoy/config"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/migrate"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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

			m := migrate.NewMigrator(c, opts, migrations)

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

			m := migrate.NewMigrator(c, opts, migrations)

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
			ID: "201601021504",
			Migrate: func(db *mongo.Database) error {
				fmt.Println("Migrating up")
				return nil
			},
			Rollback: func(db *mongo.Database) error {
				fmt.Println("Rolling back")
				return nil
			},
		},
	}
)
