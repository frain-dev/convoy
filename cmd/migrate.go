package main

import (
	"context"
	"fmt"
	"time"

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

			//o := postgres.NewOrgRepo(db.GetDB())
			//_ = o.CreateOrganisation(cmd.Context(), &datastore.Organisation{
			//	OwnerID: "xxx",
			//	Name:    "123",
			//})
			//
			//orgs, _, err := o.LoadOrganisationsPaged(cmd.Context(), datastore.Pageable{
			//	Page:    1,
			//	PerPage: 10,
			//})
			//if err != nil {
			//	fmt.Printf("orgs: %+v", err)
			//	return
			//}
			//
			//p := postgres.NewProjectRepo(db.GetDB())
			//err = p.UpdateProject(cmd.Context(), &datastore.Project{
			//	UID:            "1",
			//	Name:           "xxx-swer",
			//	Type:           datastore.IncomingProject,
			//	OrganisationID: orgs[0].UID,
			//	Config: &datastore.ProjectConfig{
			//		RateLimitCount:     1000,
			//		RateLimitDuration:  60,
			//		StrategyType:       datastore.ExponentialStrategyProvider,
			//		StrategyDuration:   100,
			//		StrategyRetryCount: 10,
			//		SignatureHeader:    config.DefaultSignatureHeader,
			//		SignatureHash:      "SHA256",
			//		RetentionPolicy:    "5d",
			//	},
			//})
			//if err != nil {
			//	fmt.Printf("err: %+v", err)
			//	return
			//}

			// proj, err := p.FetchProjectByID(cmd.Context(), 1)
			// if err != nil {
			// 	fmt.Printf("err: %+v", err)
			// 	return
			// }
			// fmt.Printf("\n%+v\n", proj)
			// fmt.Printf("\n%+v\n", proj.Config)

			// c := postgres.NewConfigRepo(db.GetDB())
			// err = c.UpdateConfiguration(cmd.Context(), &datastore.Configuration{
			// 	UID:                "default",
			// 	IsAnalyticsEnabled: true,
			// 	IsSignupEnabled:    true,
			// 	StoragePolicy: &datastore.StoragePolicyConfiguration{
			// 		Type: datastore.OnPrem,
			// 		S3: &datastore.S3Storage{
			// 			Bucket:       null.NewString("Bucket", true),
			// 			AccessKey:    null.NewString("AccessKey", true),
			// 			SecretKey:    null.NewString("SecretKey", true),
			// 			Region:       null.NewString("Region", true),
			// 			SessionToken: null.NewString("SessionToken", true),
			// 			Endpoint:     null.NewString("Endpoint", true),
			// 		},
			// 		OnPrem: datastore.DefaultStoragePolicy.OnPrem,
			// 	},
			// })
			// if err != nil {
			// 	fmt.Printf("err: %+v", err)
			// 	return
			// }

			// cfg, err := c.LoadConfiguration(cmd.Context())
			// if err != nil {
			// 	fmt.Printf("err: %+v", err)
			// 	return
			// }

			// fmt.Printf("config: %+v\n", cfg.StoragePolicy.OnPrem)
			// fmt.Printf("config: %+v\n", cfg.StoragePolicy.S3)

			// projects, err := p.LoadProjects(cmd.Context(), &datastore.ProjectFilter{OrgID: "1"})
			// for _, v := range projects {
			// 	fmt.Printf("Proj: %+v\n", v)
			// }

			u := postgres.NewUserRepo(db.GetDB())
			ctx := context.Background()
			user := &datastore.User{
				UID:                        "1",
				FirstName:                  "Daniel",
				LastName:                   "O.J",
				Email:                      "danvixent@gmail.com",
				EmailVerified:              true,
				Password:                   "32322",
				ResetPasswordToken:         "vvv",
				EmailVerificationToken:     "vvvc",
				CreatedAt:                  time.Now(),
				UpdatedAt:                  time.Now(),
				ResetPasswordExpiresAt:     time.Now(),
				EmailVerificationExpiresAt: time.Now(),
			}

			err = u.CreateUser(ctx, user)
			if err != nil {
				log.Fatal("create user", err)
			}

			user.FirstName = "jjj"
			err = u.UpdateUser(ctx, user)
			if err != nil {
				log.Fatal("update user", err)
			}

			dbUser, err := u.FindUserByID(ctx, "1")
			if err != nil {
				log.Fatal("find user", err)
			}

			fmt.Printf("%+v\n=====\n", user)
			fmt.Printf("%+v\n======\n", dbUser)
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
