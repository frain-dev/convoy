package migrate

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/migrator"
	"github.com/frain-dev/convoy/pkg/log"
)

func AddMigrateCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convoy migrations",
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
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			lo := log.NewLogger(os.Stdout)
			lo.SetLevel(log.DebugLevel)

			lo.Info("Running migrations...")

			cfg, err := config.Get()
			if err != nil {
				lo.WithError(err).Fatal("Error fetching the config.")
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				lo.Fatal(err)
			}
			defer db.Close()

			m := migrator.NewWithLogger(db, lo)
			err = m.Up()
			if err != nil {
				lo.Fatalf("migration up failed with error: %+v", err)
			}

			lo.Info("Migration completed successfully.")

			os.Exit(0)
		},
	}

	return cmd
}

func addDownCommand() *cobra.Command {
	var maxMigrations int

	cmd := &cobra.Command{
		Use:     "down",
		Aliases: []string{"migrate-down"},
		Short:   "Rollback migrations",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			lo := log.NewLogger(os.Stdout)
			lo.SetLevel(log.DebugLevel)

			cfg, err := config.Get()
			if err != nil {
				lo.WithError(err).Fatal("Error fetching the config.")
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				lo.Fatal(err)
			}
			defer db.Close()

			m := migrator.NewWithLogger(db, lo)
			err = m.Down(maxMigrations)
			if err != nil {
				lo.Fatalf("migration down failed with error: %+v", err)
			}

			lo.Info("Migration completed successfully.")

			os.Exit(0)
		},
	}

	cmd.Flags().IntVar(&maxMigrations, "max", 1, "The maximum number of migrations to rollback")

	return cmd
}

func addCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "creates a new migration file",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			lo := log.NewLogger(os.Stdout)
			lo.SetLevel(log.DebugLevel)

			fileName := fmt.Sprintf("sql/%v.sql", time.Now().Unix())
			f, err := os.Create(fileName)
			if err != nil {
				lo.Fatal(err)
			}

			defer f.Close()

			lines := []string{"-- +migrate Up", "-- +migrate Down"}
			for _, line := range lines {
				_, err := f.WriteString(line + "\n\n")
				if err != nil {
					lo.Fatal(err)
				}
			}

			lo.Infof("Created migration: %s", fileName)
		},
	}

	return cmd
}
