package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	convoyMongo "github.com/frain-dev/convoy/datastore/mongo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func addUpgradeCommand(a *app) *cobra.Command {
	var oldVersion string
	var newVersion string

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Convoy Upgrader",
		Run: func(cmd *cobra.Command, args []string) {
			switch oldVersion {
			case "v0.4":
				if newVersion == "v0.5" {
					updateVersion4ToVersion5()
				}
				log.Error(fmt.Sprintf("%s is not a valid new version for v0.4", newVersion))
			case "v0.5":
				if newVersion == "v0.6" {
					updateVersion5ToVersion6()
				}
				log.Error(fmt.Sprintf("%s is not a valid new version for v0.5", newVersion))
			default:
				log.Error(fmt.Sprintf("%s is not a valid old version", oldVersion))
			}
			os.Exit(0)
		},
	}

	cmd.Flags().StringVar(&oldVersion, "from-version", "", "old version")
	cmd.Flags().StringVar(&newVersion, "to-version", "", "new version")
	return cmd
}

func updateVersion4ToVersion5() {
	ctx := context.Background()

	cfg, err := config.Get()
	if err != nil {
		log.WithError(err).Fatalf("Error fetching the config.")
	}

	db, err := NewDB(cfg)
	if err != nil {
		log.WithError(err).Fatalf("Error connecting to the db.")
	}

	groups, err := db.GroupRepo().LoadGroups(ctx, &datastore.GroupFilter{})
	if err != nil {
		log.WithError(err).Fatalf("Error fetching the groups.")
	}

	for _, grp := range groups {
		group := *grp
		group.RateLimit = 5000
		group.RateLimitDuration = "1m"

		err = db.GroupRepo().UpdateGroup(ctx, &group)
		if err != nil {
			log.WithError(err).Fatalf("Error updating group details.")
		}
	}

	log.Info("Upgrade complete")
	os.Exit(0)
}

func updateVersion5ToVersion6() {
	cfg, err := config.Get()
	if err != nil {
		log.WithError(err).Fatalf("Error fetching the config.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := options.Client()
	opts.ApplyURI(cfg.Database.Dsn)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		log.WithError(err).Error("mongo connection error")
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		log.WithError(err)

	}

	u, err := url.Parse(cfg.Database.Dsn)
	if err != nil {
		log.WithError(err).Error("error parsing database uri")

	}

	dbName := strings.TrimPrefix(u.Path, "/")
	conn := client.Database(dbName, nil)
	collection := conn.Collection(convoyMongo.APIKeyCollection)

	update := bson.M{"$rename": bson.M{"role.groups": "role.group", "role.apps": "role.app"}}

	_, err = collection.UpdateMany(context.Background(), bson.D{}, update)
	if err != nil {
		log.WithError(err).Error("error updating many")
	}
	ops := options.Find()

	cursor, err := collection.Find(context.Background(), bson.D{}, ops)
	if err != nil {
		log.WithError(err).Error("error finding many")
	}
	for cursor.Next(context.TODO()) {
		var apiKey APIKey
		err := cursor.Decode(&apiKey)
		if err != nil {
			log.WithError(err)
		}
		filter := bson.M{"uid": apiKey.UID}
		var update primitive.D
		if len(apiKey.Role.Group) > 0 && len(apiKey.Role.App) > 0 {
			keytype := datastore.AppPortalKey
			role := auth.Role{
				Type:  apiKey.Role.Type,
				Group: apiKey.Role.Group[0],
				App:   apiKey.Role.App[0],
			}
			update = bson.D{
				primitive.E{Key: "role", Value: role},
				primitive.E{Key: "key_type", Value: keytype},
				primitive.E{Key: "updated_at", Value: primitive.NewDateTimeFromTime(time.Now())},
			}

		} else {
			keytype := datastore.ProjectKey
			role := auth.Role{
				Type:  apiKey.Role.Type,
				Group: apiKey.Role.Group[0],
			}
			update = bson.D{
				primitive.E{Key: "role", Value: role},
				primitive.E{Key: "key_type", Value: keytype},
				primitive.E{Key: "updated_at", Value: primitive.NewDateTimeFromTime(time.Now())},
			}
		}
		_, err = collection.UpdateOne(context.Background(), filter, bson.M{"$set": update})
		if err != nil {
			log.WithError(err).Error("error updating one")
		}
	}
	log.Info("Upgrade complete")
	os.Exit(0)
}

type APIKey struct {
	ID        primitive.ObjectID `json:"-" bson:"_id"`
	UID       string             `json:"uid" bson:"uid"`
	MaskID    string             `json:"mask_id,omitempty" bson:"mask_id"`
	Name      string             `json:"name" bson:"name"`
	Role      Role               `json:"role" bson:"role"`
	Hash      string             `json:"hash,omitempty" bson:"hash"`
	Salt      string             `json:"salt,omitempty" bson:"salt"`
	Type      datastore.KeyType  `json:"key_type" bson:"key_type"`
	ExpiresAt primitive.DateTime `json:"expires_at,omitempty" bson:"expires_at,omitempty"`
	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at"`

	DocumentStatus datastore.DocumentStatus `json:"-" bson:"document_status"`
}

type Role struct {
	Type  auth.RoleType `json:"type"`
	Group []string      `json:"groups"`
	App   []string      `json:"apps,omitempty"`
}
