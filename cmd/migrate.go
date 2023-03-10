package main

import (
	"fmt"
	"os"
	"time"

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
			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}

			err = config.LoadConfig(cfgPath)
			if err != nil {
				return err
			}

			_, err = config.Get()
			if err != nil {
				return err
			}

			// Override with CLI Flags
			cliConfig, err := buildCliConfiguration(cmd)
			if err != nil {
				return err
			}

			if err = config.Override(cliConfig); err != nil {
				return err
			}

			return nil

		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.AddCommand(addUpCommand())
	cmd.AddCommand(addDownCommand())
	cmd.AddCommand(addCreateCommand())

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

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}

			defer db.GetDB().Close()

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
	var max int

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

			defer db.GetDB().Close()

			m := migrator.New(db)
			err = m.Down(max)
			if err != nil {
				log.Fatalf("migration down failed with error: %+v", err)
			}
		},
	}

	cmd.Flags().IntVar(&max, "max", 1, "The maximum number of migrations to rollback")

	return cmd
}

func addCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "creates a new migration file",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			fileName := fmt.Sprintf("sql/%v.sql", time.Now().Unix())
			f, err := os.Create(fileName)
			if err != nil {
				log.Fatal(err)
			}

			defer f.Close()

			lines := []string{"-- +migrate Up", "-- +migrate Down"}
			for _, line := range lines {
				_, err := f.WriteString(line + "\n\n")
				if err != nil {
					log.Fatal(err)
				}
			}
		},
	}

	return cmd
}
