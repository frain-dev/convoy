package main

import (
	"context"
	"net/url"
	"strings"

	"github.com/frain-dev/convoy/config"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/migrate"
	"github.com/frain-dev/convoy/pkg/log"

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
		Use:     "up",
		Aliases: []string{"migrate-up"},
		Short:   "Run all pending migrations",
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

			m := migrate.NewMigrator(c, opts, migrate.Migrations, nil)

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
		Use:     "down",
		Aliases: []string{"migrate-down"},
		Short:   "Rollback migrations",
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

			m := migrate.NewMigrator(c, opts, migrate.Migrations, nil)

			err = m.RollbackTo(context.Background(), migrationID)
			if err != nil {
				log.WithError(err).Fatalf("Error rolling back migrations")
			}
		},
	}

	cmd.Flags().StringVar(&migrationID, "id", "", "Migration ID")

	return cmd
}
