package exporter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/sirupsen/logrus"
)

var (
	ErrInvalidTable     = errors.New("invalid table to export")
	ErrInvalidExportDir = errors.New("invalid export directory")
)

type tablename string

const (
	eventsTable           tablename = "convoy.events"
	eventDeliveriesTable  tablename = "convoy.event_deliveries"
	deliveryAttemptsTable tablename = "convoy.delivery_attempts"
)

// order is important here,
// delivery_attempts references the event delivery id and
// event_deliveries references event id,
// so delivery_attempts must be deleted first,
// then event_deliveries then events.
var tables = []tablename{deliveryAttemptsTable, eventDeliveriesTable, eventsTable}

var tableToFileMapping = map[tablename]string{
	eventsTable:           "%s/orgs/%s/projects/%s/events/%s.json",
	eventDeliveriesTable:  "%s/orgs/%s/projects/%s/eventdeliveries/%s.json",
	deliveryAttemptsTable: "%s/orgs/%s/projects/%s/deliveryattempts/%s.json",
}

type (
	ExportResult      map[tablename]ExportTableResult
	ExportTableResult struct {
		NumDocs    int64
		ExportFile string
	}
)

type Exporter struct {
	config  *datastore.Configuration
	project *datastore.Project

	expDate time.Time
	result  ExportResult

	// repositories
	eventRepo            datastore.EventRepository
	projectRepo          datastore.ProjectRepository
	eventDeliveryRepo    datastore.EventDeliveryRepository
	deliveryAttemptsRepo datastore.DeliveryAttemptsRepository
}

func NewExporter(projectRepo datastore.ProjectRepository,
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	p *datastore.Project, c *datastore.Configuration,
	attemptsRepo datastore.DeliveryAttemptsRepository,
) (*Exporter, error) {
	return &Exporter{
		config:  c,
		project: p,
		result:  ExportResult{},
		expDate: time.Now().Add(-time.Hour * 24),

		eventRepo:            eventRepo,
		projectRepo:          projectRepo,
		deliveryAttemptsRepo: attemptsRepo,
		eventDeliveryRepo:    eventDeliveryRepo,
	}, nil
}

func (ex *Exporter) Export(ctx context.Context) (ExportResult, error) {
	if !ex.config.RetentionPolicy.IsRetentionPolicyEnabled {
		return nil, nil
	}

	// export tables
	for _, table := range tables {
		result, err := ex.exportTable(ctx, table, ex.expDate)
		if err != nil {
			return nil, err
		}

		ex.result[table] = *result
		log.Printf("exported %v record(s) from %v", ex.result[table].NumDocs, table)
	}

	return ex.result, nil
}

func (ex *Exporter) exportTable(ctx context.Context, table tablename, expDate time.Time) (*ExportTableResult, error) {
	result := &ExportTableResult{}
	exportFileFormat, ok := tableToFileMapping[table]
	if !ok {
		return result, ErrInvalidTable
	}

	exportDir, err := ex.getExportDir()
	if err != nil {
		return result, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	exportFile := fmt.Sprintf(exportFileFormat, exportDir, ex.project.OrganisationID, ex.project.UID, now)

	writer, err := getOutputWriter(exportFile)
	if err != nil {
		return result, err
	}

	if writer == nil {
		writer = os.Stdout
	} else {
		defer writer.Close()
	}

	repo, err := ex.getRepo(table)
	if err != nil {
		return result, err
	}

	numDocs, err := repo.ExportRecords(ctx, ex.project.UID, expDate, writer)
	if err != nil {
		log.WithError(err).Error("failed to export records")
		return result, err
	}

	result.NumDocs = numDocs
	result.ExportFile = exportFile

	return result, nil
}

func (ex *Exporter) getExportDir() (string, error) {
	switch ex.config.StoragePolicy.Type {
	case datastore.S3:
		return convoy.TmpExportDir, nil
	case datastore.OnPrem:
		if ex.config.StoragePolicy.OnPrem.Path.IsZero() {
			return "", ErrInvalidExportDir
		}

		return ex.config.StoragePolicy.OnPrem.Path.String, nil
	default:
		return "", ErrInvalidExportDir
	}
}

func (ex *Exporter) getRepo(table tablename) (datastore.ExportRepository, error) {
	switch table {
	case eventsTable:
		return ex.eventRepo, nil
	case eventDeliveriesTable:
		return ex.eventDeliveryRepo, nil
	case deliveryAttemptsTable:
		return ex.deliveryAttemptsRepo, nil
	default:
		return nil, ErrInvalidTable
	}
}

// GetOutputWriter opens and returns an io.WriteCloser for the output
// options or nil if none is set. The caller is responsible for closing it.
func getOutputWriter(out string) (io.WriteCloser, error) {
	// If the directory in which the output file is to be
	// written does not exist, create it
	fileDir := filepath.Dir(out)
	err := os.MkdirAll(fileDir, 0o750)
	if err != nil {
		return nil, err
	}

	file, err := os.Create(toUniversalPath(out))
	if err != nil {
		return nil, err
	}

	return file, err
}

// type compressWriter struct {
//	gw   *gzip.Writer
//	file *os.File
// }
//
// func (c compressWriter) Write(b []byte) (int, error) {
//	return c.gw.Write(b)
// }
//
// func (c compressWriter) Close() error {
//	if err := c.gw.Close(); err != nil {
//		return err
//	}
//
//	return c.file.Close()
// }

func toUniversalPath(path string) string {
	return filepath.FromSlash(path)
}
