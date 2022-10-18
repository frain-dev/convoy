package migrate

import (
	"context"
	"fmt"
	"time"

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
					log.WithError(err).Fatalf("Failed migration")
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
					log.WithError(err).Fatalf("Failed migration")
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
					log.WithError(err).Fatalf("Failed migration")
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
					log.WithError(err).Fatalf("Failed migration")
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
					log.WithError(err).Fatalf("Failed migration")
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
					log.WithError(err).Fatalf("Failed migration")
					return err
				}
			}

			return nil
		},
	},
}
