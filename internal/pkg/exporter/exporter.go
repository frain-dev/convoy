package exporter

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	blobstore "github.com/frain-dev/convoy/internal/pkg/blob-store"
	log "github.com/frain-dev/convoy/pkg/logger"
)

var (
	ErrInvalidTable     = errors.New("invalid table to export")
	ErrInvalidExportDir = errors.New("invalid export directory")
)

type tableName string

const (
	eventsTable           tableName = "convoy.events"
	eventDeliveriesTable  tableName = "convoy.event_deliveries"
	deliveryAttemptsTable tableName = "convoy.delivery_attempts"
)

// order is important here,
// delivery_attempts references the event delivery id and
// event_deliveries references event id,
// so delivery_attempts must be deleted first,
// then event_deliveries then events.
var tables = []tableName{deliveryAttemptsTable, eventDeliveriesTable, eventsTable}

var tableToBlobKeyMapping = map[tableName]string{
	eventsTable:           "backup/%s/events/%s.jsonl.gz",
	eventDeliveriesTable:  "backup/%s/eventdeliveries/%s.jsonl.gz",
	deliveryAttemptsTable: "backup/%s/deliveryattempts/%s.jsonl.gz",
}

// tableToFileMapping is used by the disk-based Export() path.
var tableToFileMapping = map[tableName]string{
	eventsTable:           "%s/backup/%s/events/%s.jsonl.gz",
	eventDeliveriesTable:  "%s/backup/%s/eventdeliveries/%s.jsonl.gz",
	deliveryAttemptsTable: "%s/backup/%s/deliveryattempts/%s.jsonl.gz",
}

type (
	ExportResult      map[tableName]ExportTableResult
	ExportTableResult struct {
		NumDocs    int64
		ExportFile string
	}
)

type Exporter struct {
	config *datastore.Configuration

	expStart time.Time
	expEnd   time.Time
	result   ExportResult

	// repositories
	eventRepo            datastore.EventRepository
	eventDeliveryRepo    datastore.EventDeliveryRepository
	deliveryAttemptsRepo datastore.DeliveryAttemptsRepository

	logger log.Logger
}

func NewExporter(
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	c *datastore.Configuration,
	attemptsRepo datastore.DeliveryAttemptsRepository,
	logger log.Logger,
) (*Exporter, error) {
	// Derive the look back duration from CONVOY_BACKUP_INTERVAL (defaults to 1h)
	lookBackDur := DefaultBackupInterval
	if cfg, err := config.Get(); err == nil {
		lookBackDur = ParseBackupInterval(cfg.RetentionPolicy.BackupInterval)
	}

	return &Exporter{
		config:   c,
		result:   ExportResult{},
		expEnd:   time.Now().UTC(),
		expStart: time.Now().UTC().Add(-lookBackDur),

		eventRepo:            eventRepo,
		deliveryAttemptsRepo: attemptsRepo,
		eventDeliveryRepo:    eventDeliveryRepo,
		logger:               logger,
	}, nil
}

// NewExporterWithWindow creates an Exporter with an explicit time window,
// bypassing the config-derived backup interval. Used by manual/ad-hoc backups.
func NewExporterWithWindow(
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	c *datastore.Configuration,
	attemptsRepo datastore.DeliveryAttemptsRepository,
	start, end time.Time,
	logger log.Logger,
) (*Exporter, error) {
	if !start.Before(end) {
		return nil, fmt.Errorf("invalid export window: start (%s) must be before end (%s)",
			start.Format(time.RFC3339), end.Format(time.RFC3339))
	}

	return &Exporter{
		config:   c,
		result:   ExportResult{},
		expEnd:   end,
		expStart: start,

		eventRepo:            eventRepo,
		deliveryAttemptsRepo: attemptsRepo,
		eventDeliveryRepo:    eventDeliveryRepo,
		logger:               logger,
	}, nil
}

// Export writes gzip-compressed JSONL files to disk. Used by the legacy
// file-based backup flow and E2E tests.
func (ex *Exporter) Export(ctx context.Context) (ExportResult, error) {
	if !ex.config.RetentionPolicy.IsRetentionPolicyEnabled {
		return nil, nil
	}

	for _, table := range tables {
		result, err := ex.exportTableToDisk(ctx, table, ex.expStart, ex.expEnd)
		if err != nil {
			return nil, err
		}

		ex.result[table] = *result
		ex.logger.Info(fmt.Sprintf("exported %v record(s) from %v", ex.result[table].NumDocs, table))
	}

	return ex.result, nil
}

// StreamExport exports all tables and streams gzip-compressed JSONL directly to
// the given BlobStore via io.Pipe, avoiding any local disk writes.
func (ex *Exporter) StreamExport(ctx context.Context, store blobstore.BlobStore) (ExportResult, error) {
	if !ex.config.RetentionPolicy.IsRetentionPolicyEnabled {
		return nil, nil
	}

	result := ExportResult{}

	for _, table := range tables {
		tableResult, err := ex.streamExportTable(ctx, store, table, ex.expStart, ex.expEnd)
		if err != nil {
			return nil, err
		}

		result[table] = *tableResult
		ex.logger.Info(fmt.Sprintf("streamed %v record(s) from %v", tableResult.NumDocs, table))
	}

	return result, nil
}

// streamExportTable pipes ExportRecords → gzip → BlobStore.Upload without touching disk.
func (ex *Exporter) streamExportTable(ctx context.Context, store blobstore.BlobStore, table tableName, expStart, expEnd time.Time) (*ExportTableResult, error) {
	keyFormat, ok := tableToBlobKeyMapping[table]
	if !ok {
		return nil, ErrInvalidTable
	}

	now := time.Now().UTC()
	date := now.Format("2006-01-02")
	ts := now.Format(time.RFC3339)
	key := fmt.Sprintf(keyFormat, date, ts)

	exportRepo, err := ex.getRepo(table)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	exportCtx, cancelExport := context.WithCancel(ctx)
	defer cancelExport()

	var numDocs int64
	errCh := make(chan error, 1)

	go func() {
		gzw := gzip.NewWriter(pw)

		n, exportErr := exportRepo.ExportRecords(exportCtx, expStart, expEnd, gzw)
		numDocs = n

		// MUST close gzip before pipe — flush trailer (checksum + size)
		if closeErr := gzw.Close(); closeErr != nil && exportErr == nil {
			exportErr = closeErr
		}
		if exportErr != nil {
			pw.CloseWithError(exportErr)
		} else {
			pw.Close()
		}
		errCh <- exportErr
	}()

	uploadErr := store.Upload(ctx, key, pr)
	exportErr := <-errCh

	if uploadErr != nil {
		return nil, fmt.Errorf("upload %q: %w", key, uploadErr)
	}
	if exportErr != nil {
		return nil, fmt.Errorf("export %q: %w", key, exportErr)
	}

	return &ExportTableResult{NumDocs: numDocs, ExportFile: key}, nil
}

// exportTableToDisk writes gzip-compressed JSONL to a local file (legacy path).
func (ex *Exporter) exportTableToDisk(ctx context.Context, table tableName, expStart, expEnd time.Time) (*ExportTableResult, error) {
	result := &ExportTableResult{}
	exportFileFormat, ok := tableToFileMapping[table]
	if !ok {
		return result, ErrInvalidTable
	}

	exportDir, err := ex.getExportDir()
	if err != nil {
		return result, err
	}

	now := time.Now().UTC()
	date := now.Format("2006-01-02")
	ts := now.Format(time.RFC3339)
	exportFile := fmt.Sprintf(exportFileFormat, exportDir, date, ts)

	fileWriter, err := getOutputWriter(exportFile)
	if err != nil {
		return result, err
	}
	defer func(fileWriter io.WriteCloser) {
		if err = fileWriter.Close(); err != nil {
			ex.logger.Error("failed to close file writer", "error", err)
		}
	}(fileWriter)

	gzw := gzip.NewWriter(fileWriter)
	defer func(gzw *gzip.Writer) {
		if err = gzw.Close(); err != nil {
			ex.logger.Error("failed to close gzip writer", "error", err)
		}
	}(gzw)

	exportRepo, err := ex.getRepo(table)
	if err != nil {
		return result, err
	}

	numDocs, err := exportRepo.ExportRecords(ctx, expStart, expEnd, gzw)
	if err != nil {
		ex.logger.Error("failed to export records", "error", err)
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

func (ex *Exporter) getRepo(table tableName) (datastore.ExportRepository, error) {
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

func getOutputWriter(out string) (io.WriteCloser, error) {
	fileDir := filepath.Dir(out)
	err := os.MkdirAll(fileDir, 0o750)
	if err != nil {
		return nil, err
	}

	file, err := os.Create(filepath.FromSlash(out))
	if err != nil {
		return nil, err
	}

	return file, err
}
