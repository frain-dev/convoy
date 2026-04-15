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
)

func AddMigrateCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convoy migrations",
	}

	cmd.AddCommand(addUpCommand(a))
	cmd.AddCommand(addDownCommand(a))
	cmd.AddCommand(addCreateCommand(a))

	return cmd
}

func addUpCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "up",
		Aliases: []string{"migrate-up"},
		Short:   "Run all pending migrations",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a.Logger.Info("Running migrations...")

			cfg, err := config.Get()
			if err != nil {
				return err
			}

			db, err := postgres.NewDB(cfg, a.Logger)
			if err != nil {
				return err
			}
			defer db.Close()

			m := migrator.NewWithLogger(db, a.Logger)
			err = m.Up()
			if err != nil {
				return err
			}

			a.Logger.Info("Migration completed successfully.")

			return nil
		},
	}

	return cmd
}

func addDownCommand(a *cli.App) *cobra.Command {
	var maxMigrations int

	cmd := &cobra.Command{
		Use:     "down",
		Aliases: []string{"migrate-down"},
		Short:   "Rollback migrations",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a.Logger.Info("Rolling back migrations...")

			cfg, err := config.Get()
			if err != nil {
				return err
			}

			db, err := postgres.NewDB(cfg, a.Logger)
			if err != nil {
				return err
			}
			defer db.Close()

			m := migrator.NewWithLogger(db, a.Logger)
			err = m.Down(maxMigrations)
			if err != nil {
				return err
			}

			a.Logger.Info("Migration completed successfully.")
			return nil
		},
	}

	cmd.Flags().IntVar(&maxMigrations, "max", 1, "The maximum number of migrations to rollback")

	return cmd
}

func addCreateCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "creates a new migration file",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a.Logger.Info("Creating new migration file...")

			fileName := fmt.Sprintf("sql/%v.sql", time.Now().Unix())
			f, err := os.Create(fileName)
			if err != nil {
				return err
			}

			defer f.Close()

			lines := []string{"-- +migrate Up", "-- +migrate Down"}
			for _, line := range lines {
				_, err := f.WriteString(line + "\n\n")
				if err != nil {
					return err
				}
			}

			a.Logger.Infof("Created migration: %s", fileName)
			return nil
		},
	}

	return cmd
}
