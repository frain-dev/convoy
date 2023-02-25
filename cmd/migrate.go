package main

import (
	"fmt"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/migrator"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/spf13/cobra"
)

func addMigrateCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convoy migrations",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("I'm here 1 >>>>>>")
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("I'm here 2 >>>>>>")
			return nil
		},
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
			// cfg, err := config.Get()
			// if err != nil {
			// 	log.WithError(err).Fatalf("Error fetching the config.")
			// }

			cfg := config.Configuration{
				Database: config.DatabaseConfiguration{
					Dsn: "postgres://postgres@localhost/convoy?sslmode=disable",
				},
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}

			m := migrator.New(db)
			err = m.Up()
			if err != nil {
				log.Fatalf("migration up failed with error: %+v", err)
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

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}

			m := migrator.New(db)
			err = m.Down()
			if err != nil {
				log.Fatalf("migration up failed with error: %+v", err)
			}
		},
	}

	cmd.Flags().StringVar(&migrationID, "id", "", "Migration ID")

	return cmd
}
