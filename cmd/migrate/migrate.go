package migrate

import (
	"fmt"
	"github.com/frain-dev/convoy/database/sqlite3"
	"os"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/migrator"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/spf13/cobra"
)

var mapping = map[string]string{
	"agent":  "postgres",
	"server": "sqlite",
}

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
	var component string
	cmd := &cobra.Command{
		Use:     "up",
		Aliases: []string{"migrate-up"},
		Short:   "Run all pending migrations",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			t, err := cmd.Flags().GetString("component")
			if err != nil {
				log.Fatal(err)
			}

			if t != "server" && t != "agent" {
				log.Fatalf("Invalid component %s. Must be one of: server or agent", t)
			}

			switch t {
			case "server":
				cfg, err := config.Get()
				if err != nil {
					log.WithError(err).Fatal("[sqlite3] error fetching the config.")
				}

				db, err := sqlite3.NewDB(cfg.Database.SqliteDB, log.NewLogger(os.Stdout))
				if err != nil {
					log.Fatal(err)
				}

				defer db.Close()

				m := migrator.New(db, "sqlite3")
				err = m.Up()
				if err != nil {
					log.Fatalf("[sqlite3] migration up failed with error: %+v", err)
				}
			case "agent":
				cfg, err := config.Get()
				if err != nil {
					log.WithError(err).Fatal("[postgres] error fetching the config.")
				}

				db, err := postgres.NewDB(cfg)
				if err != nil {
					log.Fatal(err)
				}

				defer db.Close()

				m := migrator.New(db, "postgres")
				err = m.Up()
				if err != nil {
					log.Fatalf("[postgres] migration up failed with error: %+v", err)
				}
			}

			log.Info("migration up succeeded")
		},
	}

	cmd.Flags().StringVarP(&component, "component", "c", "server", "The component to create for: (server|agent)")

	return cmd
}

func addDownCommand() *cobra.Command {
	var maxDown int

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
				log.WithError(err).Fatal("Error fetching the config.")
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}

			defer db.Close()

			m := migrator.New(db, "postgres")
			err = m.Down(maxDown)
			if err != nil {
				log.Fatalf("migration down failed with error: %+v", err)
			}
		},
	}

	cmd.Flags().IntVar(&maxDown, "max", 1, "The maximum number of migrations to rollback")

	return cmd
}

func addCreateCommand() *cobra.Command {
	var component string
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"migrate-create"},
		Short:   "creates a new migration file",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			t, err := cmd.Flags().GetString("component")
			if err != nil {
				log.Fatal(err)
			}

			if t != "server" && t != "agent" {
				log.Fatalf("Invalid component %s. Must be one of: server or agent", t)
			}

			fileName := fmt.Sprintf("sql/%s/%v.sql", mapping[component], time.Now().Unix())
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

	cmd.Flags().StringVarP(&component, "component", "c", "server", "The component to create for: (server|agent)")

	return cmd
}
