package task

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"os"
	"time"
	"github.com/frain-dev/convoy/internal/pkg/exporter"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	objectstore "github.com/frain-dev/convoy/datastore/object-store"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/hibiken/asynq"
	"bytes"
	"compress/gzip"
	"io/ioutil"	
)

const (
	eventsTable          = "convoy.events"
	eventDeliveriesTable = "convoy.event_deliveries"
)

func RetentionPolicies(configRepo datastore.ConfigurationRepository, projectRepo datastore.ProjectRepository, eventRepo datastore.EventRepository, eventDeliveriesRepo datastore.EventDeliveryRepository, exportRepo datastore.ExportRepository, rd *rdb.Redis) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd.Client())
	rs := redsync.New(pool)

	return func(ctx context.Context, t *asynq.Task) error {
		const mutexName = "convoy:retention:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(time.Second), redsync.WithTries(1))

		tctx, cancel := context.WithTimeout(ctx, time.Second*2)
		defer cancel()

		err := mutex.LockContext(tctx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		defer func() {
			tctx, cancel := context.WithTimeout(ctx, time.Second*2)
			defer cancel()

			ok, err := mutex.UnlockContext(tctx)
			if !ok || err != nil {
				log.WithError(err).Error("failed to release lock")
			}
		}()

		c := time.Now()
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
			return err
		}

		filter := &datastore.ProjectFilter{}
		projects, err := projectRepo.LoadProjects(context.Background(), filter)
		if err != nil {
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
					err = ExportCollection(ctx, table, exportDir, expDate, objectStoreClient, p, eventRepo, eventDeliveriesRepo, projectRepo, exportRepo)
					if err != nil {
						return err
					}
				}
			}
		}
		fmt.Printf("Retention policy job took %f minutes to run", time.Since(c).Minutes())
		return nil
	}
}

func NewObjectStoreClient(storage *datastore.StoragePolicyConfiguration) (objectstore.ObjectStore, string, error) {
	switch storage.Type {
	case datastore.S3:
		exportDir := convoy.TmpExportDir
		objectStoreOpts := objectstore.ObjectStoreOptions{
			Prefix:       storage.S3.Prefix.ValueOrZero(),
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
	projectRepo datastore.ProjectRepository, exportRepo datastore.ExportRepository,
) error {
	out := GetArgsByCollection(tableName, exportDir, project)
	tempFile := out + ".tmp" // Temporary file name for compression

	dbExporter := &exporter.Exporter{
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
		return os.Remove(tempFile)
	}

	// Compress the file before uploading to the object store
	err = compressFile(out, tempFile)
	if err != nil {
		return err
	}

	// upload to object store
	err = objectStoreClient.Save(out)
	if err != nil {
		return err
	}
	// Cleanup temporary file if the upload was successful
	err = os.Remove(tempFile)
	if err != nil {
		return err
	}

	// Cleanup out file if the upload was successful
	err = os.Remove(out)
	if err != nil {
		return err
	}
	
	switch tableName {
	case eventsTable:
		eventFilter := &datastore.EventFilter{
			CreatedAtStart: 0,
			CreatedAtEnd:   expDate.Unix(),
		}
		err = eventRepo.DeleteProjectEvents(ctx, project.UID, eventFilter, true)
		if err != nil {
			return err
		}

		err = eventRepo.DeleteProjectTokenizedEvents(ctx, project.UID, eventFilter)
		if err != nil {
			return err
		}

		project.RetainedEvents += int(numDocs)
		err = projectRepo.UpdateProject(ctx, project)
		if err != nil {
			return err
		}

	case eventDeliveriesTable:
		eventDeliveryFilter := &datastore.EventDeliveryFilter{
			CreatedAtStart: 0,
			CreatedAtEnd:   expDate.Unix(),
		}
		err = eventDeliveriesRepo.DeleteProjectEventDeliveries(ctx, project.UID, eventDeliveryFilter, true)
		if err != nil {
			return err
		}
	}

	return nil
}

func compressFile(outFile, inFile string) error {
	inputBytes, err := ioutil.ReadFile(inFile)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	_, err = gzipWriter.Write(inputBytes)
	if err != nil {
		return err
	}

	err = gzipWriter.Close()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(outFile, buf.Bytes(), 0644)
	if err != nil {
		return err
	}

	return nil
}