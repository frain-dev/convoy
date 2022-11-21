package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
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
				UID            string                   `json:"uid" bson:"uid"`
				Config         Config                   `json:"config" bson:"config"`
				DocumentStatus datastore.DocumentStatus `json:"-" bson:"document_status"`
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
				UID            string                   `json:"uid" bson:"uid"`
				RetryConfig    RetryConfig              `json:"retry_config" bson:"retry_config"`
				DocumentStatus datastore.DocumentStatus `json:"-" bson:"document_status"`
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

			appCollection := "applications"
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, appCollection)

			type Application struct {
				UID       string               `json:"uid" bson:"uid"`
				Endpoints []datastore.Endpoint `json:"endpoints" bson:"endpoints"`
			}

			var apps []*Application
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
						UID:            uuid.NewString(),
						Value:          endpoint.Secret,
						CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
						UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
						DocumentStatus: datastore.ActiveDocumentStatus,
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

			appCollection := "applications"
			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, appCollection)

			type Application struct {
				UID       string               `json:"uid" bson:"uid"`
				Endpoints []datastore.Endpoint `json:"endpoints" bson:"endpoints"`
			}

			var apps []*Application
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
							ID:                 primitive.NewObjectID(),
							UID:                e.UID,
							GroupID:            app.GroupID,
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
							DocumentStatus:     e.DocumentStatus,
						}

						endpoints = append(endpoints, endpoint)
					}
				} else {
					endpoint := &datastore.Endpoint{
						ID:              primitive.NewObjectID(),
						UID:             app.UID,
						GroupID:         app.GroupID,
						Title:           app.Title,
						SupportEmail:    app.SupportEmail,
						SlackWebhookURL: app.SlackWebhookURL,
						AppID:           app.UID,
						CreatedAt:       app.CreatedAt,
						UpdatedAt:       app.UpdatedAt,
						DocumentStatus:  app.DocumentStatus,
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
			store := datastore.New(db)

			ctx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EndpointCollection)

			var endpoints []*datastore.Endpoint
			err := store.FindAll(ctx, nil, nil, nil, &endpoints)
			if err != nil {
				log.WithError(err).Fatalf("Failed to find endpoints")
				return err
			}

			mApps := make(map[string]*datastore.Application, 0)

			for _, endpoint := range endpoints {
				ap, ok := mApps[endpoint.AppID]
				endpointResp := datastore.DeprecatedEndpoint{
					UID:                endpoint.UID,
					TargetURL:          endpoint.TargetURL,
					Description:        endpoint.Description,
					Secret:             endpoint.Secret,
					Secrets:            endpoint.Secrets,
					AdvancedSignatures: endpoint.AdvancedSignatures,
					HttpTimeout:        endpoint.HttpTimeout,
					RateLimit:          endpoint.RateLimit,
					RateLimitDuration:  endpoint.RateLimitDuration,
					Authentication:     endpoint.Authentication,
					CreatedAt:          endpoint.CreatedAt,
					UpdatedAt:          endpoint.UpdatedAt,
					DocumentStatus:     endpoint.DocumentStatus,
				}

				if ok {
					ap.Endpoints = append(ap.Endpoints, endpointResp)
				} else {
					ap := &datastore.Application{
						ID:              primitive.NewObjectID(),
						UID:             endpoint.AppID,
						GroupID:         endpoint.GroupID,
						Title:           endpoint.Title,
						SupportEmail:    endpoint.SupportEmail,
						SlackWebhookURL: endpoint.SlackWebhookURL,
						IsDisabled:      endpoint.IsDisabled,
						CreatedAt:       endpoint.CreatedAt,
						UpdatedAt:       endpoint.UpdatedAt,
						DocumentStatus:  endpoint.DocumentStatus,
					}

					if !util.IsStringEmpty(endpoint.TargetURL) {
						ap.Endpoints = []datastore.DeprecatedEndpoint{endpointResp}
					}

					mApps[endpoint.AppID] = ap
				}
			}

			appCtx := context.WithValue(context.Background(), datastore.CollectionCtx, "applications")
			for _, app := range mApps {
				err := store.Save(appCtx, app, nil)
				if err != nil {
					return err
				}
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
					"$set": bson.M{
						"endpoints": nil,
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
}
