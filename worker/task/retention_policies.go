package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	objectstore "github.com/frain-dev/convoy/datastore/object-store"
	"github.com/frain-dev/convoy/util"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func RententionPolicies(instanceConfig config.Configuration, configRepo datastore.ConfigurationRepository, groupRepo datastore.GroupRepository, eventRepo datastore.EventRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		//TODO: delete files from export/tmp dir

		config, err := configRepo.LoadConfiguration(ctx)
		if err != nil {
			return err
		}
		collection := "events"
		objectStoreClient, exportDir, err := NewObjectStoreClient(config)
		if err != nil {
			return err
		}

		filter := &datastore.GroupFilter{}

		groups, err := groupRepo.LoadGroups(context.Background(), filter)
		if err != nil {
			log.WithError(err).Error("failed to load groups.")
			return err
		}

		for _, g := range groups {
			cfg := g.Config
			if cfg.IsRetentionPolicyEnabled {
				//export events collection
				policy, err := time.ParseDuration(cfg.RetentionPolicy.Policy)
				if err != nil {
					return err
				}
				expDate := primitive.NewDateTimeFromTime(time.Now().Add(-policy))
				uri := instanceConfig.Database.Dsn
				query := fmt.Sprintf("{ 'group_id': %s 'created_at': { '$lt': { '$created_at': %s }}}", g.UID, fmt.Sprint(expDate))
				out := fmt.Sprintf(exportDir, g.OrganisationID, "/", g.UID, "/", "events/", time.Now(), "/", "events.json") //<org-id>/<project-id>/events/<today-as-ISODateTime>
				args := util.MongoExportArgsBuilder(uri, collection, query, out)
				err = util.MongoExport(args)
				if err != nil {
					return err
				}

				//upload to object store
				err = objectStoreClient.Save(out)
				if err != nil {
					return err
				}

				//delete documents
				//TODO: filter by "created_at"
				err = eventRepo.DeleteGroupEvents(ctx, g.UID)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func NewObjectStoreClient(config *datastore.Configuration) (objectstore.ObjectStore, string, error) {
	//TODO: Check if exportDir exist and create it
	switch config.StoragePolicy.Type {
	case datastore.S3:
		exportDir := convoy.TmpExportDir
		objectStoreOpts := objectstore.ObjectStoreOptions{
			Bucket:       config.StoragePolicy.S3.Bucket,
			AccessKey:    config.StoragePolicy.S3.AccessKey,
			SecretKey:    config.StoragePolicy.S3.SecretKey,
			SessionToken: config.StoragePolicy.S3.SessionToken,
		}
		objectStoreClient, err := objectstore.NewS3Client(objectStoreOpts)
		if err != nil {
			return nil, "", err
		}
		return objectStoreClient, exportDir, nil

	case datastore.OnPrem:
		exportDir := config.StoragePolicy.OnPrem.Path
		objectStoreOpts := objectstore.ObjectStoreOptions{
			OnPremStorageDir: exportDir,
		}
		objectStoreClient, err := objectstore.NewOnPremClient(objectStoreOpts)
		if err != nil {
			return nil, "", err
		}
		return objectStoreClient, exportDir, nil
	default:
		return nil, "", errors.New("invalid storage policy")
	}
}
