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
	"github.com/frain-dev/convoy/searcher"
	"github.com/frain-dev/convoy/util"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

func RententionPolicies(instanceConfig config.Configuration, configRepo datastore.ConfigurationRepository, groupRepo datastore.GroupRepository, eventRepo datastore.EventRepository, searcher searcher.Searcher) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
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
				expDate := time.Now().UTC().Add(-policy)
				uri := instanceConfig.Database.Dsn
				query := fmt.Sprintf(`{ "group_id": "%s", "document_status": "Active", "created_at": { "$lt": { "$date": "%s" }}}`, g.UID, fmt.Sprint(expDate.Format(time.RFC3339)))
				out := fmt.Sprint(exportDir, g.OrganisationID, "/", g.UID, "/", "events/", time.Now().UTC().Format(time.RFC3339), "/", "events.json") //<org-id>/<project-id>/events/<today-as-ISODateTime>
				args := util.MongoExportArgsBuilder(uri, collection, query, out)
				numDocs, err := util.MongoExport(args)
				if err != nil {
					return err
				}
				if numDocs > 0 {
					//upload to object store
					err = objectStoreClient.Save(out)
					if err != nil {
						return err
					}

					//delete documents
					searchFilter := &datastore.Filter{
						Group: g,
						SearchParams: datastore.SearchParams{
							CreatedAtStart: 0,
							CreatedAtEnd:   expDate.Unix(),
						},
					}
					err = searcher.Remove(collection, searchFilter)
					if err != nil {
						return err
					}

					evntFilter := &datastore.EventFilter{
						GroupID:        g.UID,
						CreatedAtStart: 0,
						CreatedAtEnd:   expDate.Unix(),
					}
					err = eventRepo.DeleteGroupEvents(ctx, evntFilter)
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
}

func NewObjectStoreClient(config *datastore.Configuration) (objectstore.ObjectStore, string, error) {
	switch config.StoragePolicy.Type {
	case datastore.S3:
		exportDir := convoy.TmpExportDir
		objectStoreOpts := objectstore.ObjectStoreOptions{
			Bucket:       config.StoragePolicy.S3.Bucket,
			AccessKey:    config.StoragePolicy.S3.AccessKey,
			SecretKey:    config.StoragePolicy.S3.SecretKey,
			SessionToken: config.StoragePolicy.S3.SessionToken,
			Region:       config.StoragePolicy.S3.Region,
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
