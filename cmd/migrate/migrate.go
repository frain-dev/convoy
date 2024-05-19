package migrate

import (
	"fmt"
	"os"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/migrator"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/spf13/cobra"
)

func AddMigrateCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convoy migrations",
	}

	cmd.AddCommand(addUpCommand())
	cmd.AddCommand(addDownCommand())
	cmd.AddCommand(addCreateCommand())
	cmd.AddCommand(addListCommand())

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
			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Fatalf("Error fetching the config.")
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}

			defer db.Close()

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
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Fatalf("Error fetching the config.")
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}

			defer db.Close()

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
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
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

func addListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list all migrations",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Fatalf("Error fetching the config.")
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}

			defer db.Close()

			m := migrator.New(db)
			migrationRecords, err := m.List()
			if err != nil {
				log.WithError(err).Fatal("failed to list migrations")
			}

			if len(migrationRecords) == 0 {
				fmt.Println("No migrations found")
			}

			fmt.Printf("Name              Applied At\n----------------- ----------------------------\n")

			for _, record := range migrationRecords {
				fmt.Printf("%s    %v\n", record.Name, record.AppliedAt.Format("2006-01-02 15:04 (MST)"))
			}
		},
	}

	return cmd
}
