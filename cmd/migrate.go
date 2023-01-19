package main

import (
	"fmt"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/migrator"
	"github.com/frain-dev/convoy/pkg/log"

	"github.com/spf13/cobra"
)

func addMigrateCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convoy migrations",
	}

	cmd.AddCommand(addUpCommand())
	cmd.AddCommand(addDownCommand())
	cmd.AddCommand(addRunCommand())

	return cmd
}

func addRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run",
		Aliases: []string{"migrate-run"},
		Short:   "Run arbitrary SQL queries",
		Run: func(cmd *cobra.Command, args []string) {
			_, err := config.Get()
			if err != nil {
				log.WithError(err).Fatalf("Error fetching the config.")
			}

			db, err := database.New()
			if err != nil {
				log.Fatal(err)
			}

			o := postgres.NewOrgRepo(db.GetDB())
			// _ = o.CreateOrganisation(cmd.Context(), &datastore.Organisation{
			// 	OwnerID: "xxx",
			// 	Name:    "123",
			// })

			orgs, _, err := o.LoadOrganisationsPaged(cmd.Context(), datastore.Pageable{
				Page:    1,
				PerPage: 10,
			})

			if err != nil {
				fmt.Printf("orgs: %+v", err)
				return
			}

			fmt.Printf("page: %+v\n", orgs[0].UID)

			p := postgres.NewProjectRepo(db.GetDB())
			err = p.CreateProject(cmd.Context(), &datastore.Project{
				Name:           "MMM",
				Type:           datastore.IncomingProject,
				OrganisationID: orgs[0].UID,
				Config: &datastore.ProjectConfig{
					RateLimitCount:     1000,
					RateLimitDuration:  60,
					StrategyType:       datastore.LinearStrategyProvider,
					StrategyDuration:   100,
					StrategyRetryCount: 10,
					SignatureHeader:    config.DefaultSignatureHeader,
					SignatureHash:      "SHA265",
					RetentionPolicy:    "30d",
				},
			})

			if err != nil {
				fmt.Printf("err: %+v", err)
				return
			}

			// fmt.Printf("Org: %+v\n", o)
		},
	}

	return cmd
}

func addUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "up",
		Aliases: []string{"migrate-up"},
		Short:   "Run all pending migrations",
		Run: func(cmd *cobra.Command, args []string) {
			_, err := config.Get()
			if err != nil {
				log.WithError(err).Fatalf("Error fetching the config.")
			}

			db, err := database.New()
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
			_, err := config.Get()
			if err != nil {
				log.WithError(err).Fatalf("Error fetching the config.")
			}

			db, err := database.New()
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
