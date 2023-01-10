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

		filter := &datastore.ProjectFilter{}
		projects, err := projectRepo.LoadProjects(context.Background(), filter)
		if err != nil {
			log.WithError(err).Error("failed to load projects.")
			return err
		}

		for _, p := range projects {
			cfg := p.Config
			if cfg.IsRetentionPolicyEnabled {
				//export events collection
				policy, err := time.ParseDuration(cfg.RetentionPolicy.Policy)
				if err != nil {
					return err
				}
				expDate := time.Now().UTC().Add(-policy)
				uri := instanceConfig.Database.Dsn
				for _, collection := range collections {
					err = ExportCollection(ctx, collection, uri, exportDir, expDate, objectStoreClient, p, eventRepo, eventDeliveriesRepo, projectRepo, searcher)
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

func GetArgsByCollection(collection string, uri string, exportDir string, expDate time.Time, project *datastore.Project) ([]string, string, error) {
	switch collection {
	case "events":
		query := fmt.Sprintf(`{ "project_id": "%s", "deleted_at": null, "created_at": { "$lt": { "$date": "%s" }}}`, project.UID, fmt.Sprint(expDate.Format(time.RFC3339)))
		// orgs/<org-id>/projects/<project-id>/events/<today-as-ISODateTime>
		out := fmt.Sprintf("%s/orgs/%s/projects/%s/events/%s.json", exportDir, project.OrganisationID, project.UID, time.Now().UTC().Format(time.RFC3339))
		args := util.MongoExportArgsBuilder(uri, collection, query, out)
		return args, out, nil

	case "eventdeliveries":
		query := fmt.Sprintf(`{ "project_id": "%s", "deleted_at": null, "created_at": { "$lt": { "$date": "%s" }}}`, project.UID, fmt.Sprint(expDate.Format(time.RFC3339)))
		// orgs/<org-id>/projects/<project-id>/eventdeliveries/<today-as-ISODateTime>
		out := fmt.Sprintf("%s/orgs/%s/projects/%s/eventdeliveries/%s.json", exportDir, project.OrganisationID, project.UID, time.Now().UTC().Format(time.RFC3339))
		args := util.MongoExportArgsBuilder(uri, collection, query, out)
		return args, out, nil
	default:
		return nil, "", errors.New("invalid collection")
	}
}

func ExportCollection(ctx context.Context, collection string, uri string, exportDir string, expDate time.Time, objectStoreClient objectstore.ObjectStore, project *datastore.Project, eventRepo datastore.EventRepository, eventDeliveriesRepo datastore.EventDeliveryRepository, projectRepo datastore.ProjectRepository, searcher searcher.Searcher) error {
	args, out, err := GetArgsByCollection(collection, uri, exportDir, expDate, project)
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
			ProjectID:      project.UID,
			CreatedAtStart: 0,
			CreatedAtEnd:   expDate.Unix(),
		}
		err = eventRepo.DeleteProjectEvents(ctx, evntFilter, true)
		if err != nil {
			return err
		}
		projectMetadata := project.Metadata
		// update retain count
		if projectMetadata == nil {
			project.Metadata = &datastore.ProjectMetadata{
				RetainedEvents: int(numDocs),
			}
		} else {
			project.Metadata.RetainedEvents += int(numDocs)
		}
		err = projectRepo.UpdateProject(ctx, project)
		if err != nil {
			return err
		}

	case "eventdeliveries":
		evntDeliveryFilter := &datastore.EventDeliveryFilter{
			ProjectID:      project.UID,
			CreatedAtStart: 0,
			CreatedAtEnd:   expDate.Unix(),
		}
		err = eventDeliveriesRepo.DeleteProjectEventDeliveries(ctx, evntDeliveryFilter, true)
		if err != nil {
			return err
		}
	}

	// delete documents
	sf := &datastore.SearchFilter{FilterBy: datastore.FilterBy{
		ProjectID: project.UID,
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
