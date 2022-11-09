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
					log.WithError(err).Fatalf("Failed migration 20221021100029_migrate_group_signature_config_to_versions UpdateByID")
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

	{
		ID: "20221031102300_change_subscription_event_types_to_filters",
		Migrate: func(db *mongo.Database) error {
			type Subscription struct {
				UID          string                        `json:"uid" bson:"uid"`
				FilterConfig datastore.FilterConfiguration `json:"filter_config" bson:"filter_config"`
			}

			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SubscriptionCollection)

			var subscriptions []*Subscription
			err := store.FindAll(ctx, nil, nil, nil, &subscriptions)
			if err != nil {
				log.WithError(err).Fatalf("Failed migration 20220906166248_change_subscription_event_types_to_filters")
				return err
			}

			for _, s := range subscriptions {
				var filter map[string]interface{}

				if len(s.FilterConfig.EventTypes) == 1 {
					if s.FilterConfig.EventTypes[0] == "*" {
						filter = map[string]interface{}{}
					} else {
						filter = map[string]interface{}{"event_types": s.FilterConfig.EventTypes[0]}
					}
				} else {
					filter = map[string]interface{}{"event_types": map[string]interface{}{"$in": s.FilterConfig.EventTypes}}
				}

				update := bson.M{
					"$set": bson.M{
						"filter_config.filter": filter,
					},
				}

				err := store.UpdateByID(ctx, s.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20220906166248_change_subscription_event_types_to_filters")
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SubscriptionCollection)

			update := bson.M{
				"$unset": bson.M{
					"filter_config.filter": 1,
				},
			}

			err := store.UpdateMany(ctx, nil, update, true)
			if err != nil {
				log.WithError(err).Fatalf("Failed migration 20220906166248_change_subscription_event_types_to_filters rollback")
				return err
			}

			return nil
		},
	},

	{
		ID: "20221181000600_migrate_api_key_roles",
		Migrate: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.APIKeyCollection)
			type Role struct {
				Type   string   `json:"type"`
				Apps   []string `json:"apps"`
				Groups []string `json:"groups"`
			}

			type Key struct {
				UID  string `json:"uid" bson:"uid"`
				Role Role   `json:"role" bson:"role"`
			}

			fn := func(sessCtx mongo.SessionContext) error {
				var keys []Key
				err := store.FindAll(sessCtx, nil, nil, nil, &keys)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221181000600_migrate_api_key_roles rollback FindAll")
					return err
				}

				for _, key := range keys {
					update := bson.M{}
					if len(key.Role.Groups) > 0 {
						update["role.group"] = key.Role.Groups[0]
					}

					if len(key.Role.Apps) > 0 {
						update["role.app"] = key.Role.Apps[0]
					}

					err := store.UpdateByID(sessCtx, key.UID, bson.M{"$set": update})
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20221181000600_migrate_api_key_roles rollback UpdateByID")
						return err
					}
				}

				unset := bson.M{
					"$unset": bson.M{
						"role.groups": "",
						"role.apps":   "",
					},
				}

				err = store.UpdateMany(sessCtx, bson.M{}, unset, true)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221181000600_migrate_api_key_roles UpdateMany")
					return err
				}

				return nil
			}

			return store.WithTransaction(ctx, fn)
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.APIKeyCollection)
			type Role struct {
				Type  string `json:"type"`
				App   string `json:"apps"`
				Group string `json:"groups"`
			}

			type Key struct {
				UID  string `json:"uid" bson:"uid"`
				Role Role   `json:"role" bson:"role"`
			}

			fn := func(sessCtx mongo.SessionContext) error {
				var keys []Key
				err := store.FindAll(sessCtx, nil, nil, nil, &keys)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221181000600_migrate_api_key_roles FindAll")
					return err
				}

				for _, key := range keys {
					update := bson.M{
						"$set": bson.M{
							"role.groups": []string{key.Role.Group},
							"role.apps":   []string{key.Role.App},
						},
					}

					err := store.UpdateByID(sessCtx, key.UID, update)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20221181000600_migrate_api_key_roles UpdateByID")
						return err
					}
				}

				unset := bson.M{
					"$unset": bson.M{
						"role.group": "",
						"role.app":   "",
					},
				}

				err = store.UpdateMany(sessCtx, bson.M{}, unset, true)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221181000600_migrate_api_key_roles UpdateMany")
					return err
				}

				return nil
			}

			return store.WithTransaction(ctx, fn)
		},
	},
	{
		ID: "20221109100029_migrate_deprecate_document_status_field",
		Migrate: func(db *mongo.Database) error {
			collectionList := []datastore.CollectionKey{
				datastore.ConfigCollection,
				datastore.GroupCollection,
				datastore.OrganisationCollection,
				datastore.OrganisationInvitesCollection,
				datastore.OrganisationMembersCollection,
				datastore.AppCollection,
				datastore.EventCollection,
				datastore.SourceCollection,
				datastore.UserCollection,
				datastore.SubscriptionCollection,
				datastore.EventDeliveryCollection,
				datastore.APIKeyCollection,
				datastore.DeviceCollection,
			}

			for _, collectionKey := range collectionList {
				store := datastore.New(db)
				ctx := context.WithValue(context.Background(), datastore.CollectionCtx, collectionKey)

				fn := func(sessCtx mongo.SessionContext) error {
					filter := bson.M{
						"deleted_at": bson.M{
							"$or": bson.M{
								"$exists": false,
								"$lte":    primitive.NewDateTimeFromTime(time.Date(1971, 0, 0, 0, 0, 0, 0, time.UTC)),
							},
						},
					}

					set := bson.M{
						"$set": bson.M{
							"deleted_at": nil,
						},
					}

					err := store.UpdateMany(sessCtx, filter, set, true)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20221109100029_migrate_deprecate_document_status_field UpdateMany")
						return err
					}

					return nil
				}

				err := store.WithTransaction(ctx, fn)
				if err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			collectionList := []datastore.CollectionKey{
				datastore.ConfigCollection,
				datastore.GroupCollection,
				datastore.OrganisationCollection,
				datastore.OrganisationInvitesCollection,
				datastore.OrganisationMembersCollection,
				datastore.AppCollection,
				datastore.EventCollection,
				datastore.SourceCollection,
				datastore.UserCollection,
				datastore.SubscriptionCollection,
				datastore.EventDeliveryCollection,
				datastore.APIKeyCollection,
				datastore.DeviceCollection,
			}

			for _, collectionKey := range collectionList {

				store := datastore.New(db)
				ctx := context.WithValue(context.Background(), datastore.CollectionCtx, collectionKey)

				fn := func(sessCtx mongo.SessionContext) error {
					filter := bson.M{"deleted_at": nil}

					update := bson.M{"deleted_at": 0}

					err := store.UpdateMany(sessCtx, filter, update, true)
					if err != nil {
						log.WithError(err).Fatalf("Failed rollback migration 20221109100029_migrate_deprecate_document_status_field UpdateMany")
						return err
					}

					return nil
				}

				err := store.WithTransaction(ctx, fn)
				if err != nil {
					return err
				}
			}

			return nil
		},
	},
}
