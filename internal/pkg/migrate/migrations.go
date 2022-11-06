package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/sirupsen/logrus"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var Migrations = []*Migration{
	{
		ID: "20220901162904_change_group_rate_limit_configuration",
		Migrate: func(db *mongo.Database) error {
			type RTConfig struct {
				Duration string `json:"duration"`
			}

			type Config struct {
				RateLimit RTConfig `json:"ratelimit"`
			}

			type Group struct {
				UID    string `json:"uid" bson:"uid"`
				Config Config `json:"config" bson:"config"`
			}

			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.GroupCollection)

			var groups []*Group
			err := store.FindAll(ctx, nil, nil, nil, &groups)
			if err != nil {
				return err
			}

			var newDuration uint64
			for _, group := range groups {
				duration, err := time.ParseDuration(group.Config.RateLimit.Duration)
				if err != nil {
					// Set default when an error occurs.
					newDuration = datastore.DefaultRateLimitConfig.Duration
				} else {
					newDuration = uint64(duration.Seconds())
				}

				update := bson.M{
					"$set": bson.M{
						"config.ratelimit.duration": newDuration,
					},
				}
				err = store.UpdateByID(ctx, group.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20220901162904_change_group_rate_limit_configuration")
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.GroupCollection)

			var groups []*datastore.Group
			err := store.FindAll(ctx, nil, nil, nil, &groups)
			if err != nil {
				return err
			}

			var newDuration time.Duration
			for _, group := range groups {
				duration := fmt.Sprintf("%ds", group.Config.RateLimit.Duration)
				newDuration, err = time.ParseDuration(duration)
				if err != nil {
					return err
				}

				update := bson.M{
					"$set": bson.M{
						"config.ratelimit.duration": newDuration,
					},
				}
				err = store.UpdateByID(ctx, group.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20220901162904_change_group_rate_limit_configuration rollback")
					return err
				}
			}

			return nil
		},
	},

	{
		ID: "20220906166248_change_subscription_retry_configuration",
		Migrate: func(db *mongo.Database) error {
			type RetryConfig struct {
				Duration string `json:"duration"`
			}

			type Subscription struct {
				UID         string      `json:"uid" bson:"uid"`
				RetryConfig RetryConfig `json:"retry_config" bson:"retry_config"`
			}

			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SubscriptionCollection)

			var subscriptions []*Subscription
			err := store.FindAll(ctx, nil, nil, nil, &subscriptions)
			if err != nil {
				return err
			}

			var newDuration uint64
			for _, subscription := range subscriptions {
				duration, err := time.ParseDuration(subscription.RetryConfig.Duration)
				if err != nil {
					newDuration = datastore.DefaultStrategyConfig.Duration
				} else {
					newDuration = uint64(duration.Seconds())
				}

				update := bson.M{
					"$set": bson.M{
						"retry_config.duration": newDuration,
					},
				}

				err = store.UpdateByID(ctx, subscription.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20220906166248_change_subscription_retry_configuration")
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SubscriptionCollection)

			var subscriptions []*datastore.Subscription
			err := store.FindAll(ctx, nil, nil, nil, &subscriptions)
			if err != nil {
				return err
			}

			var newDuration time.Duration
			for _, subscription := range subscriptions {
				duration := fmt.Sprintf("%ds", subscription.RetryConfig.Duration)
				newDuration, err = time.ParseDuration(duration)
				if err != nil {
					return err
				}

				update := bson.M{
					"$set": bson.M{
						"retry_config.duration": newDuration.String(),
					},
				}
				err = store.UpdateByID(ctx, subscription.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20220906166248_change_subscription_retry_configuration rollback")
					return err
				}
			}
			return nil
		},
	},

	{
		ID: "20220919100029_add_default_group_configuration",
		Migrate: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.GroupCollection)

			var groups []*datastore.Group
			err := store.FindAll(ctx, nil, nil, nil, &groups)
			if err != nil {
				return err
			}

			for _, group := range groups {
				config := group.Config

				if config != nil {
					continue
				}

				config = &datastore.GroupConfig{
					Signature:       datastore.GetDefaultSignatureConfig(),
					Strategy:        &datastore.DefaultStrategyConfig,
					RateLimit:       &datastore.DefaultRateLimitConfig,
					RetentionPolicy: &datastore.DefaultRetentionPolicy,
				}

				update := bson.M{
					"$set": bson.M{
						"config": config,
					},
				}
				err = store.UpdateByID(ctx, group.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20220919100029_add_default_group_configuration")
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.GroupCollection)

			var groups []*datastore.Group
			err := store.FindAll(ctx, nil, nil, nil, &groups)
			if err != nil {
				return err
			}

			for _, group := range groups {
				config := group.Config

				if config == nil {
					continue
				}

				update := bson.M{
					"$set": bson.M{
						"config": nil,
					},
				}
				err = store.UpdateByID(ctx, group.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20220919100029_add_default_group_configuration rollback")
					return err
				}
			}

			return nil
		},
	},

	{
		ID: "20221019100029_move_secret_fields_to_secrets",
		Migrate: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.AppCollection)

			var apps []*datastore.Application
			err := store.FindAll(ctx, nil, nil, nil, &apps)
			if err != nil {
				return err
			}

			for _, app := range apps {
				for i := range app.Endpoints {
					endpoint := &app.Endpoints[i]
					if endpoint.Secret == "" {
						continue
					}

					endpoint.Secrets = append(endpoint.Secrets, datastore.Secret{
						UID:       uuid.NewString(),
						Value:     endpoint.Secret,
						CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
						UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
					})
					// endpoint.Secret = ""
					endpoint.AdvancedSignatures = false // explicitly set default
				}

				update := bson.M{
					"$set": bson.M{
						"endpoints": app.Endpoints,
					},
				}

				err = store.UpdateByID(ctx, app.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221019100029_move_secret_fields_to_secrets")
					return err
				}

				log.Printf("%+v updated", app.UID)
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.AppCollection)

			var apps []*datastore.Application
			err := store.FindAll(ctx, nil, nil, nil, &apps)
			if err != nil {
				return err
			}

			for _, app := range apps {
				for i := range app.Endpoints {
					endpoint := &app.Endpoints[i]
					if len(endpoint.Secrets) == 0 {
						continue
					}

					endpoint.Secret = endpoint.Secrets[len(endpoint.Secrets)].Value // TODO(daniel): len(endpoint.Secrets) or 0?
					endpoint.Secrets = nil
					endpoint.AdvancedSignatures = false // explicitly set default
				}

				update := bson.M{
					"$set": bson.M{
						"endpoints": app.Endpoints,
					},
				}

				err = store.UpdateByID(ctx, app.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221019100029_move_secret_fields_to_secrets rollback")
					return err
				}
			}

			return nil
		},
	},

	{
		ID: "20221021100029_migrate_group_signature_config_to_versions",
		Migrate: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.GroupCollection)

			fn := func(sessCtx mongo.SessionContext) error {
				var groups []*datastore.Group
				err := store.FindAll(sessCtx, nil, nil, nil, &groups)
				if err != nil {
					return err
				}

				for _, group := range groups {
					if len(group.Config.Signature.Versions) > 0 {
						continue
					}

					group.Config.Signature.Versions = []datastore.SignatureVersion{
						{
							UID:       uuid.NewString(),
							Hash:      group.Config.Signature.Hash,
							Encoding:  datastore.HexEncoding,
							CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
						},
					}

					update := bson.M{
						"$set": bson.M{
							"config": group.Config,
						},
					}

					err = store.UpdateByID(sessCtx, group.UID, update)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20221021100029_migrate_group_signature_config_to_versions UpdateByID")
						return err
					}
				}

				unset := bson.M{
					"$unset": bson.M{
						"config.signature.hash":     "",
						"config.signature.encoding": "",
					},
				}

				err = store.UpdateMany(sessCtx, bson.M{}, unset, true)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221021100029_migrate_group_signature_config_to_versions UpdateMany")
					return err
				}

				return nil
			}
			return store.WithTransaction(ctx, fn)
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.GroupCollection)

			fn := func(sessCtx mongo.SessionContext) error {
				var groups []*datastore.Group
				err := store.FindAll(sessCtx, nil, nil, nil, &groups)
				if err != nil {
					return err
				}

				for _, group := range groups {
					if len(group.Config.Signature.Versions) == 0 {
						continue
					}

					group.Config.Signature.Hash = group.Config.Signature.Versions[0].Hash
					group.Config.Signature.Versions = nil

					update := bson.M{
						"$set": bson.M{
							"config": group.Config,
						},
					}

					err = store.UpdateByID(sessCtx, group.UID, update)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20221021100029_migrate_group_signature_config_to_versions rollback")
						return err
					}

					unset := bson.M{
						"$unset": bson.M{
							"config.signature.versions": 1,
						},
					}

					err = store.UpdateMany(sessCtx, bson.M{}, unset, false)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration")
						return err
					}
				}

				return nil
			}

			return store.WithTransaction(ctx, fn)
		},
	},
}
