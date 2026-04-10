package task

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/datastore"
	blobstore "github.com/frain-dev/convoy/internal/pkg/blob-store"
	"github.com/frain-dev/convoy/internal/pkg/exporter"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// EnqueueBackupJobs runs hourly. It inserts a pending backup_job row for each
// (project, hour) pair and reclaims any stale claimed jobs.
func EnqueueBackupJobs(
	configRepo datastore.ConfigurationRepository,
	backupJobRepo datastore.BackupJobRepository,
	logger log.Logger,
) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		dbConfig, err := configRepo.LoadConfiguration(ctx)
		if err != nil {
			return err
		}

		if !dbConfig.RetentionPolicy.IsRetentionPolicyEnabled {
			return nil
		}

		// Enqueue a global backup job only if no job is currently pending or claimed.
		// Completed/failed jobs are kept for audit — they don't block new jobs.
		now := time.Now().UTC()
		if err = backupJobRepo.EnqueueBackupJobIfIdle(ctx, now); err != nil {
			logger.Error(fmt.Sprintf("failed to enqueue backup job: %v", err))
		}

		// Reclaim jobs that have been stuck in 'claimed' for > 30 minutes
		reclaimed, err := backupJobRepo.ReclaimStaleJobs(ctx, 30)
		if err != nil {
			logger.Error(fmt.Sprintf("failed to reclaim stale jobs: %v", err))
		} else if reclaimed > 0 {
			logger.Info(fmt.Sprintf("reclaimed %d stale backup jobs", reclaimed))
		}

		return nil
	}
}

// ProcessBackupJob claims a pending backup job and streams the export to blob
// storage. Each worker instance calls this independently — SELECT FOR UPDATE
// SKIP LOCKED ensures exactly-once processing.
func ProcessBackupJob(
	configRepo datastore.ConfigurationRepository,
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	attemptsRepo datastore.DeliveryAttemptsRepository,
	backupJobRepo datastore.BackupJobRepository,
	logger log.Logger,
) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		dbConfig, err := configRepo.LoadConfiguration(ctx)
		if err != nil {
			return err
		}

		if !dbConfig.RetentionPolicy.IsRetentionPolicyEnabled {
			return nil
		}

		workerID := generateWorkerID()

		// Claim the next pending job (returns nil if none available)
		job, err := backupJobRepo.ClaimBackupJob(ctx, workerID)
		if err != nil {
			return fmt.Errorf("claim backup job: %w", err)
		}
		if job == nil {
			return nil // no work available
		}

		logger.Info(fmt.Sprintf("processing backup job %s [%s, %s)",
			job.ID, job.HourStart.Format(time.RFC3339), job.HourEnd.Format(time.RFC3339)))

		// Create blob store client
		blobStoreClient, err := blobstore.NewBlobStoreClient(dbConfig.StoragePolicy, logger)
		if err != nil {
			_ = backupJobRepo.FailBackupJob(ctx, job.ID, fmt.Sprintf("create blob store: %v", err))
			return err
		}

		// Create exporter and stream to blob storage
		e, err := exporter.NewExporter(eventRepo, eventDeliveryRepo, dbConfig, attemptsRepo, logger)
		if err != nil {
			_ = backupJobRepo.FailBackupJob(ctx, job.ID, fmt.Sprintf("create exporter: %v", err))
			return err
		}

		result, err := e.StreamExport(ctx, blobStoreClient)
		if err != nil {
			_ = backupJobRepo.FailBackupJob(ctx, job.ID, fmt.Sprintf("stream export: %v", err))
			return err
		}

		// Collect record counts
		counts := make(map[string]int64)
		for table, r := range result {
			counts[string(table)] = r.NumDocs
		}

		if err := backupJobRepo.CompleteBackupJob(ctx, job.ID, counts); err != nil {
			return fmt.Errorf("complete backup job: %w", err)
		}

		logger.Info(fmt.Sprintf("completed backup job %s", job.ID))
		return nil
	}
}

// ManualBackup runs a one-time backup with an explicit time window.
// It always uses the cron-based Exporter, never CDC, regardless of config.
func ManualBackup(
	configRepo datastore.ConfigurationRepository,
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	attemptsRepo datastore.DeliveryAttemptsRepository,
	logger log.Logger,
) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload struct {
			Start time.Time `json:"start"`
			End   time.Time `json:"end"`
		}

		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("decode manual backup payload: %w", err)
		}

		dbConfig, err := configRepo.LoadConfiguration(ctx)
		if err != nil {
			return fmt.Errorf("load configuration: %w", err)
		}

		store, err := blobstore.NewBlobStoreClient(dbConfig.StoragePolicy, logger)
		if err != nil {
			return fmt.Errorf("create blob store: %w", err)
		}

		exp, err := exporter.NewExporterWithWindow(
			eventRepo, eventDeliveryRepo, dbConfig, attemptsRepo,
			payload.Start, payload.End, logger,
		)
		if err != nil {
			return fmt.Errorf("create exporter: %w", err)
		}

		result, err := exp.StreamExport(ctx, store)
		if err != nil {
			return fmt.Errorf("stream export: %w", err)
		}

		for table, r := range result {
			logger.Info(fmt.Sprintf("manual backup: %s — %d records → %s", table, r.NumDocs, r.ExportFile))
		}

		return nil
	}
}

func generateWorkerID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}
