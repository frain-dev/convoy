package task

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/exporter"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	objectstore "github.com/frain-dev/convoy/datastore/object-store"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/hibiken/asynq"
)

const (
	eventsTable          = "convoy.events"
	eventDeliveriesTable = "convoy.event_deliveries"
)

func RetentionPolicies(configRepo datastore.ConfigurationRepository, projectRepo datastore.ProjectRepository, eventRepo datastore.EventRepository, eventDeliveriesRepo datastore.EventDeliveryRepository, exportRepo datastore.ExportRepository, searcher searcher.Searcher) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		config, err := configRepo.LoadConfiguration(ctx)
		if err != nil {
			if errors.Is(err, datastore.ErrConfigNotFound) {
				return nil
			}
			return err
		}

		// order is important here, event_deliveries references
		// event id, so event_deliveries must be deleted first
		tables := []string{eventDeliveriesTable, eventsTable}

		objectStoreClient, exportDir, err := NewObjectStoreClient(config.StoragePolicy)
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
				// export tables
				policy, err := time.ParseDuration(cfg.RetentionPolicy.Policy)
				if err != nil {
					return err
				}
				expDate := time.Now().UTC().Add(-policy)
				for _, table := range tables {
					err = ExportCollection(ctx, table, exportDir, expDate, objectStoreClient, p, eventRepo, eventDeliveriesRepo, projectRepo, exportRepo, searcher)
					if err != nil {
						log.WithError(err).Errorf("Error exporting table %v", table)
						return err
					}
				}
			}
		}
		return nil
	}
}

func NewObjectStoreClient(storage *datastore.StoragePolicyConfiguration) (objectstore.ObjectStore, string, error) {
	switch storage.Type {
	case datastore.S3:
		exportDir := convoy.TmpExportDir
		objectStoreOpts := objectstore.ObjectStoreOptions{
			Bucket:       storage.S3.Bucket.ValueOrZero(),
			Endpoint:     storage.S3.Endpoint.ValueOrZero(),
			AccessKey:    storage.S3.AccessKey.ValueOrZero(),
			SecretKey:    storage.S3.SecretKey.ValueOrZero(),
			SessionToken: storage.S3.SessionToken.ValueOrZero(),
			Region:       storage.S3.Region.ValueOrZero(),
		}
		objectStoreClient, err := objectstore.NewS3Client(objectStoreOpts)
		if err != nil {
			return nil, "", err
		}
		return objectStoreClient, exportDir, nil

	case datastore.OnPrem:
		exportDir := storage.OnPrem.Path
		objectStoreOpts := objectstore.ObjectStoreOptions{
			OnPremStorageDir: exportDir.String,
		}
		objectStoreClient, err := objectstore.NewOnPremClient(objectStoreOpts)
		if err != nil {
			return nil, "", err
		}
		return objectStoreClient, exportDir.String, nil
	default:
		return nil, "", errors.New("invalid storage policy")
	}
}

func GetArgsByCollection(tableName string, exportDir string, project *datastore.Project) string {
	switch tableName {
	case eventsTable:
		// orgs/<org-id>/projects/<project-id>/events/<today-as-ISODateTime>
		return fmt.Sprintf("%s/orgs/%s/projects/%s/events/%s.json", exportDir, project.OrganisationID, project.UID, time.Now().UTC().Format(time.RFC3339))
	case eventDeliveriesTable:
		// orgs/<org-id>/projects/<project-id>/eventdeliveries/<today-as-ISODateTime>
		return fmt.Sprintf("%s/orgs/%s/projects/%s/eventdeliveries/%s.json", exportDir, project.OrganisationID, project.UID, time.Now().UTC().Format(time.RFC3339))
	default:
		return ""
	}
}

func ExportCollection(
	ctx context.Context, tableName string, exportDir string, expDate time.Time,
	objectStoreClient objectstore.ObjectStore, project *datastore.Project,
	eventRepo datastore.EventRepository, eventDeliveriesRepo datastore.EventDeliveryRepository,
	projectRepo datastore.ProjectRepository, exportRepo datastore.ExportRepository, searcher searcher.Searcher,
) error {
	out := GetArgsByCollection(tableName, exportDir, project)

	dbExporter := &exporter.MongoExporter{
		TableName: tableName,
		ProjectID: project.UID,
		CreatedAt: expDate,
		Out:       out,
	}

	numDocs, err := dbExporter.Export(ctx, exportRepo)
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

	switch tableName {
	case eventsTable:
		evntFilter := &datastore.EventFilter{
			ProjectID:      project.UID,
			CreatedAtStart: 0,
			CreatedAtEnd:   expDate.Unix(),
		}
		err = eventRepo.DeleteProjectEvents(ctx, evntFilter, true)
		if err != nil {
			return err
		}

		project.RetainedEvents += int(numDocs)
		err = projectRepo.UpdateProject(ctx, project)
		if err != nil {
			return err
		}

	case eventDeliveriesTable:
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

	err = searcher.Remove(tableName, sf)
	if err != nil {
		log.WithError(err).Error("typesense: an error occured deleting typesense record")
	}

	return nil
}
