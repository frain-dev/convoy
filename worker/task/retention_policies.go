package task

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	objectstore "github.com/frain-dev/convoy/datastore/object-store"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/hibiken/asynq"
)

func RententionPolicies(instanceConfig config.Configuration, configRepo datastore.ConfigurationRepository, projectRepo datastore.ProjectRepository, eventRepo datastore.EventRepository, eventDeliveriesRepo datastore.EventDeliveryRepository, searcher searcher.Searcher) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		config, err := configRepo.LoadConfiguration(ctx)
		if err != nil {
			if errors.Is(err, datastore.ErrConfigNotFound) {
				return nil
			}
			return err
		}
		collections := []string{"events", "eventdeliveries"}

		objectStoreClient, exportDir, err := NewObjectStoreClient(config)
		if err != nil {
			log.WithError(err)
			return err
		}

		filter := &datastore.GroupFilter{}
		groups, err := projectRepo.LoadProjects(context.Background(), filter)
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
				for _, collection := range collections {
					err = ExportCollection(ctx, collection, uri, exportDir, expDate, objectStoreClient, g, eventRepo, eventDeliveriesRepo, projectRepo, searcher)
					if err != nil {
						log.WithError(err).Errorf("Error exporting collection %v", collection)
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
			Endpoint:     config.StoragePolicy.S3.Endpoint,
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

func GetArgsByCollection(collection string, uri string, exportDir string, expDate time.Time, group *datastore.Project) ([]string, string, error) {
	switch collection {
	case "events":
		query := fmt.Sprintf(`{ "group_id": "%s", "deleted_at": null, "created_at": { "$lt": { "$date": "%s" }}}`, group.UID, fmt.Sprint(expDate.Format(time.RFC3339)))
		// orgs/<org-id>/projects/<project-id>/events/<today-as-ISODateTime>
		out := fmt.Sprintf("%s/orgs/%s/projects/%s/events/%s.json", exportDir, group.OrganisationID, group.UID, time.Now().UTC().Format(time.RFC3339))
		args := util.MongoExportArgsBuilder(uri, collection, query, out)
		return args, out, nil

	case "eventdeliveries":
		query := fmt.Sprintf(`{ "group_id": "%s", "deleted_at": null, "created_at": { "$lt": { "$date": "%s" }}}`, group.UID, fmt.Sprint(expDate.Format(time.RFC3339)))
		// orgs/<org-id>/projects/<project-id>/eventdeliveries/<today-as-ISODateTime>
		out := fmt.Sprintf("%s/orgs/%s/projects/%s/eventdeliveries/%s.json", exportDir, group.OrganisationID, group.UID, time.Now().UTC().Format(time.RFC3339))
		args := util.MongoExportArgsBuilder(uri, collection, query, out)
		return args, out, nil
	default:
		return nil, "", errors.New("invalid collection")
	}
}

func ExportCollection(ctx context.Context, collection string, uri string, exportDir string, expDate time.Time, objectStoreClient objectstore.ObjectStore, group *datastore.Project, eventRepo datastore.EventRepository, eventDeliveriesRepo datastore.EventDeliveryRepository, projectRepo datastore.ProjectRepository, searcher searcher.Searcher) error {
	args, out, err := GetArgsByCollection(collection, uri, exportDir, expDate, group)
	if err != nil {
		return err
	}

	mongoExporter := &util.MongoExporter{Args: args}
	numDocs, err := mongoExporter.Export()
	if err != nil {
		return err
	}

	if numDocs == 0 {
		log.Printf("there is nothing to backup, will remove temp file at %s", out)
		return os.Remove(out)
	}

	// upload to object store
	err = objectStoreClient.Save(out)
	if err != nil {
		return err
	}

	switch collection {
	case "events":
		evntFilter := &datastore.EventFilter{
			GroupID:        group.UID,
			CreatedAtStart: 0,
			CreatedAtEnd:   expDate.Unix(),
		}
		err = eventRepo.DeleteGroupEvents(ctx, evntFilter, true)
		if err != nil {
			return err
		}
		groupMetaData := group.Metadata
		// update retain count
		if groupMetaData == nil {
			group.Metadata = &datastore.GroupMetadata{
				RetainedEvents: int(numDocs),
			}
		} else {
			group.Metadata.RetainedEvents += int(numDocs)
		}
		err = projectRepo.UpdateProject(ctx, group)
		if err != nil {
			return err
		}

	case "eventdeliveries":
		evntDeliveryFilter := &datastore.EventDeliveryFilter{
			GroupID:        group.UID,
			CreatedAtStart: 0,
			CreatedAtEnd:   expDate.Unix(),
		}
		err = eventDeliveriesRepo.DeleteGroupEventDeliveries(ctx, evntDeliveryFilter, true)
		if err != nil {
			return err
		}
	}

	// delete documents
	sf := &datastore.SearchFilter{FilterBy: datastore.FilterBy{
		GroupID: group.UID,
		SearchParams: datastore.SearchParams{
			CreatedAtStart: 0,
			CreatedAtEnd:   expDate.Unix(),
		},
	}}

	err = searcher.Remove(collection, sf)
	if err != nil {
		log.WithError(err).Error("typesense: an error occured deleting typesense record")
	}

	return nil
}
