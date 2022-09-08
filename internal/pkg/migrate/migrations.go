package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
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

			store := datastore.New(db, cm.GroupCollection)

			var groups []*Group
			err := store.FindAll(context.Background(), nil, nil, nil, &groups)
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

				update := bson.M{"config.ratelimit.duration": newDuration}
				err = store.UpdateByID(context.Background(), group.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration")
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {

			store := datastore.New(db, cm.GroupCollection)

			var groups []*datastore.Group
			err := store.FindAll(context.Background(), nil, nil, nil, &groups)
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

				update := bson.M{"config.ratelimit.duration": newDuration.String()}
				err = store.UpdateByID(context.Background(), group.UID, update)
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

			store := datastore.New(db, cm.SubscriptionCollection)

			var subscriptions []*Subscription
			err := store.FindAll(context.Background(), nil, nil, nil, &subscriptions)
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

				update := bson.M{"retry_config.duration": newDuration}
				err = store.UpdateByID(context.Background(), subscription.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration")
					return err
				}
			}

			return nil
		},
		Rollback: func(db *mongo.Database) error {
			store := datastore.New(db, cm.SubscriptionCollection)

			var subscriptions []*datastore.Subscription
			err := store.FindAll(context.Background(), nil, nil, nil, &subscriptions)
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
				update := bson.M{"retry_config.duration": newDuration.String()}
				err = store.UpdateByID(context.Background(), subscription.UID, update)
				if err != nil {
					log.WithError(err).Fatalf("Failed migration")
					return err
				}
			}
			return nil
		},
	},
}
