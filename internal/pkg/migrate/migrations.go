package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/google/uuid"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	appCollection    = "applications"
	groupsCollection = "groups"
)

var Migrations = []*Migration{
	{
		ID: "20220901162904_change_group_rate_limit_configuration",
		Migrate: func(db *mongo.Database) error {
			type RTConfig struct {
				Duration string `json:"duration"`
			}

			type Config struct {
				RateLimit *RTConfig `json:"ratelimit"`
			}

			type Project struct {
				UID    string  `json:"uid" bson:"uid"`
				Config *Config `json:"config" bson:"config"`
			}

			store := datastore.New(db)

			fn := func(sessCtx mongo.SessionContext) error {
				ctx := context.WithValue(sessCtx, datastore.CollectionCtx, groupsCollection)
				var projects []*Project
				err := store.FindAll(ctx, nil, nil, nil, &projects)
				if err != nil {
					return err
				}

				var newDuration uint64
				for _, project := range projects {
					if project.Config == nil || project.Config.RateLimit == nil {
						continue
					}

					duration, err := time.ParseDuration(project.Config.RateLimit.Duration)
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
					err = store.UpdateByID(ctx, project.UID, update)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20220901162904_change_group_rate_limit_configuration")
						return err
					}
				}

				return nil
			}

			return store.WithTransaction(context.Background(), fn)
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)

			fn := func(sessCtx mongo.SessionContext) error {
				ctx := context.WithValue(sessCtx, datastore.CollectionCtx, groupsCollection)
				var projects []*datastore.Project
				err := store.FindAll(ctx, nil, nil, nil, &projects)
				if err != nil {
					return err
				}

				log.Printf("%+v\n", 1)
				var newDuration time.Duration
				for _, project := range projects {

					if project.Config == nil || project.Config.RateLimit == nil {
						continue
					}

					duration := fmt.Sprintf("%ds", project.Config.RateLimit.Duration)
					newDuration, err = time.ParseDuration(duration)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20220901162904_change_group_rate_limit_configuration ParseDuration")
						return err
					}

					update := bson.M{
						"$set": bson.M{
							"config.ratelimit.duration": newDuration,
						},
					}
					err = store.UpdateByID(ctx, project.UID, update)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20220901162904_change_group_rate_limit_configuration rollback")
						return err
					}
				}

				return nil
			}

			return store.WithTransaction(context.Background(), fn)
		},
	},

	{
		ID: "20220906166248_change_subscription_retry_configuration",
		Migrate: func(db *mongo.Database) error {
			type RetryConfig struct {
				Duration string `json:"duration"`
			}

			type Subscription struct {
				UID         string       `json:"uid" bson:"uid"`
				RetryConfig *RetryConfig `json:"retry_config" bson:"retry_config"`
			}

			store := datastore.New(db)
			fn := func(sessCtx mongo.SessionContext) error {
				ctx := context.WithValue(sessCtx, datastore.CollectionCtx, datastore.SubscriptionCollection)

				var subscriptions []*Subscription
				err := store.FindAll(ctx, nil, nil, nil, &subscriptions)
				if err != nil {
					return err
				}

				var newDuration uint64
				for _, subscription := range subscriptions {
					if subscription.RetryConfig == nil {
						continue
					}

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
			}

			return store.WithTransaction(context.Background(), fn)
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)

			type RetryConfig struct {
				Type       string      `json:"type,omitempty" bson:"type,omitempty"`
				Duration   interface{} `json:"duration,omitempty" bson:"duration,omitempty"`
				RetryCount uint64      `json:"retry_count" bson:"retry_count"`
			}

			type Subscription struct {
				ID          primitive.ObjectID `json:"-" bson:"_id"`
				UID         string             `json:"uid" bson:"uid"`
				RetryConfig *RetryConfig       `json:"retry_config,omitempty" bson:"retry_config,omitempty"`
			}

			fn := func(sessCtx mongo.SessionContext) error {
				ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SubscriptionCollection)
				var subscriptions []*Subscription
				err := store.FindAll(ctx, nil, nil, nil, &subscriptions)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20220906166248_change_subscription_retry_configuration FindAll")
					return err
				}

				var newDuration time.Duration
				for _, subscription := range subscriptions {
					if subscription.RetryConfig == nil {
						continue
					}

					if subscription.RetryConfig.Duration == nil {
						continue
					}

					if _, ok := subscription.RetryConfig.Duration.(string); ok {
						continue
					}

					duration := fmt.Sprintf("%ds", subscription.RetryConfig.Duration)
					newDuration, err = time.ParseDuration(duration)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20220906166248_change_subscription_retry_configuration ParseDuration")
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
			}

			return store.WithTransaction(context.Background(), fn)
		},
	},

	{
		ID: "20220919100029_add_default_group_configuration",
		Migrate: func(db *mongo.Database) error {
			store := datastore.New(db)

			fn := func(sessCtx mongo.SessionContext) error {
				ctx := context.WithValue(sessCtx, datastore.CollectionCtx, groupsCollection)

				var projects []*datastore.Project
				err := store.FindAll(ctx, nil, nil, nil, &projects)
				if err != nil {
					return err
				}

				for _, project := range projects {
					config := project.Config

					if config != nil {
						continue
					}

					config = &datastore.ProjectConfig{
						Signature: datastore.GetDefaultSignatureConfig(),
						Strategy:  &datastore.DefaultStrategyConfig,
						RateLimit: &datastore.DefaultRateLimitConfig,
						// RetentionPolicy: &datastore.DefaultRetentionPolicy,
					}

					update := bson.M{
						"$set": bson.M{
							"config": config,
						},
					}
					err = store.UpdateByID(ctx, project.UID, update)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20220919100029_add_default_group_configuration")
						return err
					}
				}

				return nil
			}

			return store.WithTransaction(context.Background(), fn)
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)

			fn := func(sessCtx mongo.SessionContext) error {
				ctx := context.WithValue(sessCtx, datastore.CollectionCtx, groupsCollection)

				var projects []*datastore.Project
				err := store.FindAll(ctx, nil, nil, nil, &projects)
				if err != nil {
					return err
				}

				for _, project := range projects {
					config := project.Config

					if config == nil {
						continue
					}

					update := bson.M{
						"$set": bson.M{
							"config": nil,
						},
					}

					err = store.UpdateByID(ctx, project.UID, update)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20220919100029_add_default_group_configuration rollback")
						return err
					}
				}

				return nil
			}
			return store.WithTransaction(context.Background(), fn)
		},
	},

	{
		ID: "20221019100029_move_secret_fields_to_secrets",
		Migrate: func(db *mongo.Database) error {
			store := datastore.New(db)
			fn := func(sessCtx mongo.SessionContext) error {
				ctx := context.WithValue(sessCtx, datastore.CollectionCtx, appCollection)

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
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						})
						endpoint.AdvancedSignatures = false
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
			}
			return store.WithTransaction(context.Background(), fn)
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)

			fn := func(sessCtx mongo.SessionContext) error {
				ctx := context.WithValue(sessCtx, datastore.CollectionCtx, appCollection)

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

						index := 0
						if len(endpoint.Secrets) > 0 {
							index = len(endpoint.Secrets) - 1
						}

						endpoint.Secret = endpoint.Secrets[index].Value
						endpoint.Secrets = nil
						endpoint.AdvancedSignatures = false
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
			}

			return store.WithTransaction(context.Background(), fn)
		},
	},

	{
		ID: "20221021100029_migrate_group_signature_config_to_versions",
		Migrate: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, groupsCollection)

			fn := func(sessCtx mongo.SessionContext) error {
				var projects []*datastore.Project
				err := store.FindAll(sessCtx, nil, nil, nil, &projects)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221021100029_migrate_group_signature_config_to_versions UpdateByID")
					return err
				}

				for _, project := range projects {
					if len(project.Config.Signature.Versions) > 0 {
						continue
					}

					project.Config.Signature.Versions = []datastore.SignatureVersion{
						{
							UID:       uuid.NewString(),
							Hash:      project.Config.Signature.Hash,
							Encoding:  datastore.HexEncoding,
							CreatedAt: time.Now(),
						},
					}

					update := bson.M{
						"$set": bson.M{
							"config": project.Config,
						},
					}

					err = store.UpdateByID(sessCtx, project.UID, update)
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
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, groupsCollection)

			fn := func(sessCtx mongo.SessionContext) error {
				var projects []*datastore.Project
				err := store.FindAll(sessCtx, nil, nil, nil, &projects)
				if err != nil {
					return err
				}

				for _, project := range projects {
					if len(project.Config.Signature.Versions) == 0 {
						continue
					}

					project.Config.Signature.Hash = project.Config.Signature.Versions[0].Hash
					project.Config.Signature.Versions = nil

					update := bson.M{
						"$set": bson.M{
							"config": project.Config,
						},
					}

					err = store.UpdateByID(sessCtx, project.UID, update)
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
		ID: "20221109100029_migrate_deprecate_document_status_field",
		Migrate: func(db *mongo.Database) error {
			collectionList := []string{
				datastore.ConfigCollection,
				groupsCollection,
				datastore.OrganisationCollection,
				datastore.OrganisationInvitesCollection,
				datastore.OrganisationMembersCollection,
				appCollection,
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

				filter := bson.M{
					"$or": []interface{}{
						bson.D{{Key: "deleted_at", Value: bson.M{"$exists": false}}},
						bson.D{{Key: "deleted_at", Value: bson.M{"$lte": primitive.NewDateTimeFromTime(time.Date(1971, 0, 0, 0, 0, 0, 0, time.UTC))}}},
					},
				}

				set := bson.M{
					"$set": bson.M{
						"deleted_at": nil,
					},
				}

				err := store.UpdateMany(ctx, filter, set, true)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221109100029_migrate_deprecate_document_status_field UpdateMany")
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			collectionList := []string{
				datastore.ConfigCollection,
				groupsCollection,
				datastore.OrganisationCollection,
				datastore.OrganisationInvitesCollection,
				datastore.OrganisationMembersCollection,
				appCollection,
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

				filter := bson.M{"deleted_at": nil}

				update := bson.M{
					"$unset": bson.M{
						"deleted_at": "",
					},
					"$set": bson.M{
						"document_status": "Active",
					},
				}

				err := store.UpdateMany(ctx, filter, update, true)
				if err != nil {
					log.WithError(err).Fatalf("Failed rollback migration 20221109100029_migrate_deprecate_document_status_field UpdateMany")
					return err
				}
			}

			return nil
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

			err := store.UpdateMany(ctx, bson.M{}, update, true)
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
				Groups []string `json:"groups"` // Won't change this to Projects, will affect migration
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
					update := bson.M{}
					if len(key.Role.Groups) > 0 {
						update["role.group"] = key.Role.Groups[0]
					}

					if len(key.Role.Apps) > 0 {
						update["role.app"] = key.Role.Apps[0]
					}

					_, err := db.Collection(datastore.APIKeyCollection).
						UpdateOne(sessCtx, bson.M{"uid": key.UID}, bson.M{"$set": update})
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20221181000600_migrate_api_key_roles UpdateByID")
						return err
					}
				}

				unset := bson.M{
					"$unset": bson.M{
						"role.groups": "",
						"role.apps":   "",
					},
				}

				var ops []mongo.WriteModel
				updateMessagesOperation := mongo.NewUpdateManyModel()
				updateMessagesOperation.SetFilter(bson.M{})
				updateMessagesOperation.SetUpdate(unset)
				ops = append(ops, updateMessagesOperation)

				res, err := db.Collection(datastore.APIKeyCollection).BulkWrite(sessCtx, ops)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221181000600_migrate_api_key_roles - BulkWrite")
					return err
				}

				log.Infof("\n[mongodb]: results of update %s op: %+v\n", datastore.APIKeyCollection, res)

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
					log.WithError(err).Fatalf("Failed migration 20221181000600_migrate_api_key_roles rollback - FindAll")
					return err
				}

				for _, key := range keys {
					update := bson.M{
						"$set": bson.M{
							"role.groups": []string{key.Role.Group},
							"role.apps":   []string{key.Role.App},
						},
					}

					_, err := db.Collection(datastore.APIKeyCollection).
						UpdateOne(sessCtx, bson.M{"uid": key.UID}, update)
					if err != nil {
						log.WithError(err).Fatalf("Failed migration 20221181000600_migrate_api_key_roles rollback - UpdateByID")
						return err
					}
				}

				unset := bson.M{
					"$unset": bson.M{
						"role.group": "",
						"role.app":   "",
					},
				}

				var ops []mongo.WriteModel
				updateMessagesOperation := mongo.NewUpdateManyModel()
				updateMessagesOperation.SetFilter(bson.M{})
				updateMessagesOperation.SetUpdate(unset)
				ops = append(ops, updateMessagesOperation)

				res, err := db.Collection(datastore.APIKeyCollection).BulkWrite(sessCtx, ops)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221181000600_migrate_api_key_roles rollback - BulkWrite")
					return err
				}

				log.Infof("\n[mongodb]: results of update %s op: %+v\n", datastore.APIKeyCollection, res)

				return nil
			}

			return store.WithTransaction(ctx, fn)
		},
	},

	{
		ID: "20221116142027_migrate_apps_to_endpoints",
		Migrate: func(db *mongo.Database) error {
			store := datastore.New(db)

			appCollection := "applications"
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, appCollection)

			var apps []*datastore.Application
			var endpoints []*datastore.Endpoint

			err := store.FindAll(ctx, nil, nil, nil, &apps)
			if err != nil {
				log.WithError(err).Fatalf("Failed to find apps")
				return err
			}

			for _, app := range apps {
				if len(app.Endpoints) > 0 {
					for _, e := range app.Endpoints {
						endpoint := &datastore.Endpoint{
							// ID:                 primitive.NewObjectID(),
							UID:                e.UID,
							ProjectID:          app.ProjectID,
							TargetURL:          e.TargetURL,
							Title:              app.Title,
							SupportEmail:       app.SupportEmail,
							Secrets:            e.Secrets,
							AdvancedSignatures: e.AdvancedSignatures,
							Description:        e.Description,
							SlackWebhookURL:    app.SlackWebhookURL,
							AppID:              app.UID,
							HttpTimeout:        e.HttpTimeout,
							RateLimit:          e.RateLimit,
							RateLimitDuration:  e.RateLimitDuration,
							Authentication:     e.Authentication,
							CreatedAt:          e.CreatedAt,
							UpdatedAt:          e.UpdatedAt,
						}

						endpoints = append(endpoints, endpoint)
					}
				} else {
					endpoint := &datastore.Endpoint{
						// ID:              primitive.NewObjectID(),
						UID:             app.UID,
						ProjectID:       app.ProjectID,
						Title:           app.Title,
						SupportEmail:    app.SupportEmail,
						SlackWebhookURL: app.SlackWebhookURL,
						AppID:           app.UID,
						CreatedAt:       app.CreatedAt,
						UpdatedAt:       app.UpdatedAt,
					}

					endpoints = append(endpoints, endpoint)
				}
			}

			endpointCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EndpointCollection)
			for _, endpoint := range endpoints {
				err := store.Save(endpointCtx, endpoint, nil)
				if err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			err := db.Collection(datastore.EndpointCollection).Drop(context.Background())
			if err != nil {
				return err
			}

			return nil
		},
	},

	{
		ID: "20221117161319_migrate_app_events_to_endpoints",
		Migrate: func(db *mongo.Database) error {
			store := datastore.New(db)
			endpointCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EndpointCollection)
			eventCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EventCollection)

			var endpoints []*datastore.Endpoint

			err := store.FindAll(endpointCtx, nil, nil, nil, &endpoints)
			if err != nil {
				log.WithError(err).Fatalf("Failed to find endpoints")
				return err
			}

			endpointIDs := make(map[string][]string, 0)
			for _, endpoint := range endpoints {
				item, ok := endpointIDs[endpoint.AppID]
				if ok {
					item = append(item, endpoint.UID)
					endpointIDs[endpoint.AppID] = item
				}

				if !ok {
					endpointIDs[endpoint.AppID] = []string{endpoint.UID}
				}
			}

			for appID, endpointID := range endpointIDs {
				filter := bson.M{"app_id": appID}
				update := bson.M{
					"$set": bson.M{
						"endpoints": endpointID,
					},
				}
				err := store.UpdateMany(eventCtx, filter, update, true)
				if err != nil {
					log.WithError(err).Fatalf("Failed to update events")
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)
			endpointCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EndpointCollection)
			eventCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EventCollection)

			var endpoints []*datastore.Endpoint

			err := store.FindAll(endpointCtx, nil, nil, nil, &endpoints)
			if err != nil {
				log.WithError(err).Fatalf("Failed to find endpoints")
				return err
			}

			endpointIDs := make(map[string][]string, 0)
			for _, endpoint := range endpoints {
				item, ok := endpointIDs[endpoint.AppID]
				if ok {
					item = append(item, endpoint.UID)
					endpointIDs[endpoint.AppID] = item
				}

				if !ok {
					endpointIDs[endpoint.AppID] = []string{endpoint.UID}
				}
			}

			for appID := range endpointIDs {
				filter := bson.M{"app_id": appID}
				update := bson.M{
					"$unset": bson.M{
						"endpoints": "",
					},
				}
				err := store.UpdateMany(eventCtx, filter, update, true)
				if err != nil {
					log.WithError(err).Fatalf("Failed to update events")
					return err
				}
			}

			return nil
		},
	},

	{
		ID: "20221123174732_drop_devices_collection",
		Migrate: func(db *mongo.Database) error {
			// We need to drop the devices collection to successfully
			// rebuild the indexes scoped to the endpointID
			err := db.Collection(datastore.DeviceCollection).Drop(context.Background())
			if err != nil {
				log.WithError(err).Fatalf("Failed to drop devices collection")
				return err
			}
			return nil
		},
		Rollback: func(db *mongo.Database) error {
			return nil
		},
	},

	{
		ID: "20221206102519_migrate_group_id_to_project_id",
		Migrate: func(db *mongo.Database) error {
			collectionList := []string{
				datastore.ConfigCollection,
				datastore.OrganisationCollection,
				datastore.OrganisationInvitesCollection,
				datastore.OrganisationMembersCollection,
				datastore.EndpointCollection,
				datastore.EventCollection,
				datastore.SourceCollection,
				datastore.UserCollection,
				datastore.SubscriptionCollection,
				datastore.FilterCollection,
				datastore.DataMigrationsCollection,
				datastore.EventDeliveryCollection,
				datastore.APIKeyCollection,
				datastore.DeviceCollection,
				datastore.PortalLinkCollection,
			}

			for _, collectionKey := range collectionList {
				store := datastore.New(db)
				ctx := context.WithValue(context.Background(), datastore.CollectionCtx, collectionKey)

				items := bson.D{
					{Key: "group_id", Value: "project_id"},
				}

				// migrate roles
				switch collectionKey {
				case datastore.APIKeyCollection,
					datastore.UserCollection,
					datastore.OrganisationMembersCollection,
					datastore.OrganisationInvitesCollection:
					items = append(items, bson.E{Key: "role.group", Value: "role.project"})
				}

				update := bson.M{
					"$rename": items,
				}

				err := store.UpdateMany(ctx, bson.M{}, update, true)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221206102519_migrate_group_id_to_project_id UpdateMany")
					return err
				}
			}

			var projects []interface{}
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, "groups")
			store := datastore.New(db)

			err := store.FindAll(ctx, bson.M{}, nil, nil, &projects)
			if err != nil {
				log.WithError(err).Fatalf("Failed migration 20221206102519_migrate_group_id_to_project_id find all groups")
				return err
			}

			if len(projects) > 0 {
				ctx = context.WithValue(context.Background(), datastore.CollectionCtx, datastore.ProjectsCollection)
				err = store.SaveMany(ctx, projects)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration 20221206102519_migrate_group_id_to_project_id save all projects")
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			collectionList := []string{
				datastore.ConfigCollection,
				datastore.OrganisationCollection,
				datastore.OrganisationInvitesCollection,
				datastore.OrganisationMembersCollection,
				datastore.EndpointCollection,
				datastore.EventCollection,
				datastore.SourceCollection,
				datastore.UserCollection,
				datastore.SubscriptionCollection,
				datastore.FilterCollection,
				datastore.DataMigrationsCollection,
				datastore.EventDeliveryCollection,
				datastore.APIKeyCollection,
				datastore.DeviceCollection,
				datastore.PortalLinkCollection,
			}

			for _, collectionKey := range collectionList {
				store := datastore.New(db)
				ctx := context.WithValue(context.Background(), datastore.CollectionCtx, collectionKey)

				update := bson.M{
					"$rename": bson.D{
						{Key: "group_id", Value: "project_id"},
					},
				}

				err := store.UpdateMany(ctx, bson.M{}, update, true)
				if err != nil {
					log.WithError(err).Fatalf("Failed rollback migration 20221206102519_migrate_group_id_to_project_id UpdateMany")
					return err
				}
			}

			err := db.Collection(datastore.ProjectsCollection).Drop(context.Background())
			if err != nil {
				log.WithError(err).Fatalf("Failed to drop projects collection")
				return err
			}

			return nil
		},
	},

	{
		ID: "20221221112519_subscription_status_to_endpoint_status",
		Migrate: func(db *mongo.Database) error {
			store := datastore.New(db)
			subscriptionCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SubscriptionCollection)

			type Subscription struct {
				ProjectID  string `bson:"project_id"`
				EndpointID string `bson:"endpoint_id"`
				Status     string `bson:"status"`
			}

			type Endpoint struct {
				UID        string `bson:"uid"`
				IsDisabled bool   `bson:"is_disabled"`
			}

			var subscriptions []*Subscription

			err := store.FindAll(subscriptionCtx, nil, nil, nil, &subscriptions)
			if err != nil {
				log.WithError(err).Fatalf("20221221112519_subscription_status_to_endpoint_status Failed to find subscriptions")
				return err
			}

			for _, s := range subscriptions {
				var endpointStatus datastore.EndpointStatus

				switch s.Status {
				case "active":
					var e Endpoint
					endpointCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EndpointCollection)
					err := store.FindByID(endpointCtx, s.EndpointID, nil, &e)
					if err != nil {
						log.WithError(err).Errorf("20221221112519_subscription_status_to_endpoint_status Failed to find endpoint with id: %s", s.EndpointID)
						continue
					}

					if e.IsDisabled {
						endpointStatus = datastore.InactiveEndpointStatus
					} else {
						endpointStatus = datastore.ActiveEndpointStatus
					}
				case "inactive":
					endpointStatus = datastore.InactiveEndpointStatus
				case "pending":
					endpointStatus = datastore.PendingEndpointStatus
				default:
					endpointStatus = datastore.ActiveEndpointStatus
				}

				err = cm.NewEndpointRepo(store).UpdateEndpointStatus(context.Background(), s.ProjectID, s.EndpointID, endpointStatus)
				if err != nil {
					log.WithError(err).Fatalf("20221221112519_subscription_status_to_endpoint_status Failed to update endpoint status")
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db)
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EndpointCollection)

			update := bson.M{
				"$unset": bson.M{
					"status": "",
				},
			}

			err := store.UpdateMany(ctx, bson.M{}, update, true)
			if err != nil {
				log.WithError(err).Fatalf("Failed rollback migration 20221221112519_subscription_status_to_endpoint_status UpdateMany")
				return err
			}

			return nil
		},
	},
}
