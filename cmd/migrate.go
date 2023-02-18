package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/migrator"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/oklog/ulid/v2"
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

func genID(str string) {
	// t := time.Now()
	// entropy := ulid.Monotonic(crand.Reader, 0)
	ids := make([]ulid.ULID, 10_000_000)
	// ids := make([]ksuid.KSUID, 100_000_000)
	for i := range ids {
		ids[i] = ulid.Make()
		// ids[i] = ulid.MustNew(ulid.Timestamp(t), entropy)
		// ids[i] = ksuid.New()
	}
	seen := make(map[ulid.ULID]bool)
	// seen := make(map[ksuid.KSUID]bool)
	for _, id := range ids {
		fmt.Printf("%v: %v\n", str, id)
		if seen[id] {
			log.Fatal("dup")
		}
		seen[id] = true
	}
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

			ur := postgres.NewUserRepo(db.GetDB())
			user := &datastore.User{
				FirstName:                  "Daniel",
				LastName:                   "O.J",
				Email:                      "danvix",
				EmailVerified:              true,
				Password:                   "gdffiyrei",
				ResetPasswordToken:         "bfuyudy",
				EmailVerificationToken:     "vvfedfef",
				CreatedAt:                  time.Now(),
				UpdatedAt:                  time.Now(),
				DeletedAt:                  null.Time{},
				ResetPasswordExpiresAt:     time.Time{},
				EmailVerificationExpiresAt: time.Time{},
			}

			err = ur.CreateUser(cmd.Context(), user)
			if err != nil {
				log.Fatal("carete user", err)
			}

			o := postgres.NewOrgRepo(db.GetDB())
			org := &datastore.Organisation{
				UID:     ulid.Make().String(),
				OwnerID: user.UID,
				Name:    fmt.Sprintf("org-name"),
			}

			err = o.CreateOrganisation(cmd.Context(), org)
			if err != nil {
				log.Fatal("create org", err)
			}

			// orgs, _, err := o.LoadOrganisationsPaged(cmd.Context(), datastore.Pageable{
			// 	Page:    1,
			// 	PerPage: 10,
			// })

			// if err != nil {
			// 	return
			// }

			p := postgres.NewProjectRepo(db.GetDB())
			proj := &datastore.Project{
				Name:           "mob psycho",
				Type:           datastore.OutgoingProject,
				OrganisationID: org.UID,
				Config: &datastore.ProjectConfig{
					RateLimitCount:     1000,
					RateLimitDuration:  60,
					StrategyType:       datastore.ExponentialStrategyProvider,
					StrategyDuration:   100,
					StrategyRetryCount: 10,
					SignatureHeader:    config.DefaultSignatureHeader,
					RetentionPolicy:    "500d",
					SignatureVersions: []datastore.SignatureVersion{
						{
							Hash:     "SHA256",
							Encoding: datastore.HexEncoding,
						},
						{
							Hash:     "SHA512",
							Encoding: datastore.Base64Encoding,
						},
					},
				},
			}

			err = p.CreateProject(cmd.Context(), proj)
			if err != nil {
				fmt.Printf("CreateProject: %+v", err)
				return
			}

			endpoint := &datastore.Endpoint{
				ProjectID: proj.UID,
				OwnerID:   "owner1",
				TargetURL: "http://localhost",
				Title:     "test_endpoint",
				Secrets: []datastore.Secret{
					{
						UID:       ulid.Make().String(),
						Value:     "secret1",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				},
				Description:       "testing",
				HttpTimeout:       "10s",
				RateLimit:         100,
				Status:            datastore.ActiveEndpointStatus,
				RateLimitDuration: "3s",
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
			}

			endpointRepo := postgres.NewEndpointRepo(db.GetDB())
			err = endpointRepo.CreateEndpoint(context.Background(), endpoint, proj.UID)
			if err != nil {
				fmt.Printf("CreateEndpoint: %+v", err)
				return
			}

			s := postgres.NewSubscriptionRepo(db.GetDB())
			sub := &datastore.Subscription{
				Name:        "test_sub",
				Type:        datastore.SubscriptionTypeAPI,
				ProjectID:   proj.UID,
				EndpointID:  endpoint.UID,
				AlertConfig: &datastore.DefaultAlertConfig,
				RetryConfig: &datastore.DefaultRetryConfig,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"*"},
					Filter:     datastore.FilterSchema{},
				},
				RateLimitConfig: &datastore.DefaultRateLimitConfig,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}

			err = s.CreateSubscription(context.Background(), proj.UID, sub)
			if err != nil {
				fmt.Printf("CreateSubscription: %+v", err)
				return
			}

			e := postgres.NewEventRepo(db.GetDB())
			event := &datastore.Event{
				EventType: "*",
				ProjectID: proj.UID,
				Endpoints: []string{endpoint.UID},
				Data:      []byte(`{"ref":"terf"}`),
				Raw:       `{"ref":"terf"}`,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			err = e.CreateEvent(context.Background(), event)
			if err != nil {
				fmt.Printf("CreateEvent: %+v", err)
				return
			}

			now := time.Now()
			edRepo := postgres.NewEventDeliveryRepo(db.GetDB())

			for i := 0; i < 500; i++ {

				n := rand.Intn(200)
				now = now.Add(-time.Hour * 24)

				for j := 0; j < n; j++ {

					delivery := &datastore.EventDelivery{
						ProjectID:      proj.UID,
						EventID:        event.UID,
						EndpointID:     endpoint.UID,
						SubscriptionID: sub.UID,
						Status:         datastore.SuccessEventStatus,
						Metadata: &datastore.Metadata{
							Data:            event.Data,
							Raw:             event.Raw,
							Strategy:        sub.RetryConfig.Type,
							NextSendTime:    time.Now(),
							NumTrials:       1,
							IntervalSeconds: 2,
							RetryLimit:      2,
						},
						Description: "ccc",
						CreatedAt:   now,
						UpdatedAt:   now,
					}

					fmt.Println("now", now.Format(time.RFC3339))

					err = edRepo.CreateEventDelivery(context.Background(), delivery)
					if err != nil {
						fmt.Printf("CreateEventDelivery: %+v", err)
						return
					}
				}
			}

			// proj, err := p.FetchProjectByID(cmd.Context(), 9)
			// if err != nil {
			// 	fmt.Printf("err: %+v", err)
			// 	return
			// }
			// fmt.Printf("\n%+v\n", proj)
			// fmt.Printf("\n%+v\n", proj.Config)
			// fmt.Printf("\n%+v\n", proj.Config.SignatureVersions)

			//source := &datastore.Source{
			//	ProjectID:  v.UID,
			//	MaskID:     "refr9439",
			//	Name:       "test",
			//	Type:       datastore.HTTPSource,
			//	Provider:   datastore.GithubSourceProvider,
			//	IsDisabled: true,
			//	Verifier: &datastore.VerifierConfig{
			//		Type: datastore.HMacVerifier,
			//		HMac: &datastore.HMac{
			//			Header:   "h_header",
			//			Hash:     "hashed",
			//			Secret:   "3232",
			//			Encoding: datastore.Base64Encoding,
			//		},
			//		BasicAuth: nil,
			//		ApiKey:    nil,
			//	},
			//	ProviderConfig: nil,
			//	ForwardHeaders: []string{"3e2232"},
			//	CreatedAt:      time.Now(),
			//	UpdatedAt:      time.Now(),
			//}
			//fmt.Println("333")
			//sr := postgres.NewSourceRepo(db.GetDB())
			//err = sr.CreateSource(context.Background(), source)
			//if err != nil {
			//	log.Fatal("create source ", err)
			//}
			//fmt.Println("555")
			//
			//dbSource, err := sr.FindSourceByID(context.Background(), "1", source.UID)
			//if err != nil {
			//	log.Fatal("find source ", err)
			//}
			//
			//dbSource.MaskID = "47348347387837878"
			//err = sr.UpdateSource(context.Background(), "", dbSource)
			//if err != nil {
			//	log.Fatal("update source ", err)
			//}
			//
			//dbSource3, err := sr.FindSourceByID(context.Background(), "1", source.UID)
			//if err != nil {
			//	log.Fatal("find source ", err)
			//}
			//
			//pretty.Pdiff(log.NewLogger(os.Stdout), dbSource, dbSource3)

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

			// u := postgres.NewUserRepo(db.GetDB())
			// ctx := context.Background()
			// user := &datastore.User{
			// 	UID:                        "1",
			// 	FirstName:                  "Daniel",
			// 	LastName:                   "O.J",
			// 	Email:                      "danvixent@gmail.com",
			// 	EmailVerified:              true,
			// 	Password:                   "32322",
			// 	ResetPasswordToken:         "vvv",
			// 	EmailVerificationToken:     "vvvc",
			// 	CreatedAt:                  time.Now(),
			// 	UpdatedAt:                  time.Now(),
			// 	ResetPasswordExpiresAt:     time.Now(),
			// 	EmailVerificationExpiresAt: time.Now(),
			// }

			//err = u.CreateUser(ctx, user)
			//if err != nil {
			//	log.Fatal("create user", err)
			//}
			////
			////user.FirstName = "jjj"
			////err = u.UpdateUser(ctx, user)
			////if err != nil {
			////	log.Fatal("update user", err)
			////}
			//
			//dbUser, err := u.FindUserByID(ctx, "1")
			//if err != nil {
			//	log.Fatal("find user", err)
			//}
			//
			//fmt.Printf("%+v\n=====\n", user)
			//fmt.Printf("%+v\n======\n", dbUser)

			// ap := postgres.NewAPIKeyRepo(db.GetDB())
			// maskID, key := util.GenerateAPIKey()

			// salt, err := util.GenerateSecret()
			// if err != nil {
			// 	log.Fatal("failed to generate salt", err)
			// }

			// dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
			// encodedKey := base64.URLEncoding.EncodeToString(dk)

			// apiKey := &datastore.APIKey{
			// 	UID:    "1",
			// 	MaskID: maskID,
			// 	Name:   "oll",
			// 	Type:   datastore.ProjectKey, // TODO: this should be set to datastore.ProjectKey
			// 	Role: auth.Role{
			// 		Type:     auth.RoleAdmin,
			// 		Project:  "123444",
			// 		Endpoint: "dvdvdv",
			// 	},
			// 	UserID:    "1",
			// 	Hash:      encodedKey,
			// 	Salt:      salt,
			// 	CreatedAt: time.Now(),
			// 	UpdatedAt: time.Now(),
			// }

			// err = ap.CreateAPIKey(ctx, apiKey)
			// if err != nil {
			// 	log.Fatal("create api key", err)
			// }

			// apiKey.Role = auth.Role{
			// 	Type:     auth.RoleSuperUser,
			// 	Project:  "fhfhf",
			// 	Endpoint: "ffdsfds",
			// }

			// err = ap.UpdateAPIKey(ctx, apiKey)
			// if err != nil {
			// 	log.Fatal("update api key", err)
			// }

			// dbkey, err := ap.FindAPIKeyByID(ctx, apiKey.UID)
			// if err != nil {
			// 	log.Fatal("update api key", err)
			// }

			// fmt.Printf("%+v\n=====\n", apiKey)
			// fmt.Printf("%+v\n======\n", dbkey)

			// fmt.Println((*apiKey) == (*dbkey))
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
