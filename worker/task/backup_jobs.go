package task

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	blobstore "github.com/frain-dev/convoy/internal/pkg/blob-store"
	"github.com/frain-dev/convoy/internal/pkg/exporter"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const (
	backupExportMutexName  = "convoy:backup:mutex"
	backupExportMaxRuntime = 2 * time.Hour
	// backupExportDeadline bounds a single StreamExport. Must stay strictly
	// below backupClaimStaleMinutes so ReclaimStaleJobs cannot return a still-
	// running claim to pending before FailBackupJob / CompleteBackupJob.
	backupExportDeadline    = 30 * time.Minute
	backupClaimStaleMinutes = 45
)

// EnqueueBackupJobs runs on the configured backup interval. It inserts a
// pending backup_job row when idle and reclaims any stale claimed jobs.
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

		if !dbConfig.GetWebhookArchivingConfig().Enabled {
			// Still reclaim stale claims so a disable mid-flight cannot leave
			// claimed rows blocking EnqueueBackupJobIfIdle forever.
			if _, err := backupJobRepo.ReclaimStaleJobs(ctx, backupClaimStaleMinutes); err != nil {
				logger.Error(fmt.Sprintf("failed to reclaim stale backup jobs: %v", err))
			}
			return nil
		}

		storageOK := true
		if err := blobstore.StoragePolicyUsable(dbConfig.StoragePolicy); err != nil {
			logger.Warn(fmt.Sprintf("skipping backup enqueue: storage not usable: %v", err))
			storageOK = false
		}

		// Enqueue a global backup job only if no job is currently pending or claimed.
		// Completed/failed jobs are kept for audit — they don't block new jobs.
		if storageOK {
			end := time.Now().UTC()
			backupInterval := exporter.DefaultBackupInterval
			if envCfg, cfgErr := config.Get(); cfgErr == nil {
				backupInterval = exporter.ParseBackupInterval(envCfg.WebhookArchiving.Interval)
			}
			start := end.Add(-backupInterval)
			if err = backupJobRepo.EnqueueBackupJobIfIdle(ctx, start, end); err != nil {
				logger.Error(fmt.Sprintf("failed to enqueue backup job: %v", err))
			}
		}

		// Reclaim jobs stuck in 'claimed' longer than the export deadline headroom.
		reclaimed, err := backupJobRepo.ReclaimStaleJobs(ctx, backupClaimStaleMinutes)
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
// SKIP LOCKED ensures exactly-once processing. An instance-wide mutex plus a
// hard export deadline keep concurrent agents from starving the DB pool.
func ProcessBackupJob(
	configRepo datastore.ConfigurationRepository,
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	attemptsRepo datastore.DeliveryAttemptsRepository,
	backupJobRepo datastore.BackupJobRepository,
	locker JobLocker,
	logger log.Logger,
) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		return skipIfLockBusy(locker.WithLock(ctx, backupExportMutexName, backupExportMaxRuntime, func(ctx context.Context) error {
			return processBackupJob(ctx, configRepo, eventRepo, eventDeliveryRepo, attemptsRepo, backupJobRepo, logger)
		}))
	}
}

func processBackupJob(
	ctx context.Context,
	configRepo datastore.ConfigurationRepository,
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	attemptsRepo datastore.DeliveryAttemptsRepository,
	backupJobRepo datastore.BackupJobRepository,
	logger log.Logger,
) error {
	agentID := generateAgentID()

	job, err := backupJobRepo.ClaimBackupJob(ctx, agentID)
	if err != nil {
		return fmt.Errorf("claim backup job: %w", err)
	}
	if job == nil {
		return nil
	}

	logger.Info(fmt.Sprintf("processing backup job %s [%s, %s)",
		job.ID, job.HourStart.Format(time.RFC3339), job.HourEnd.Format(time.RFC3339)))

	// Load config after claim so mid-flight dashboard toggles apply to this job.
	// Do not Fail on load error: leave the claim for reclaim so a transient DB
	// blip does not terminal-fail the window.
	dbConfig, err := configRepo.LoadConfiguration(ctx)
	if err != nil {
		return fmt.Errorf("load configuration after claim %s: %w", job.ID, err)
	}

	// Fail claimed work when archiving or storage is no longer usable so pending
	// rows do not sit forever after a mid-flight config change.
	if !dbConfig.GetWebhookArchivingConfig().Enabled {
		reason := "webhook archiving is not enabled"
		if failErr := backupJobRepo.FailBackupJob(ctx, job.ID, reason); failErr != nil {
			return fmt.Errorf("fail backup job %s after archiving disabled: %w", job.ID, failErr)
		}
		logger.Warn(fmt.Sprintf("failed backup job %s: %s", job.ID, reason))
		return nil
	}

	if err := blobstore.StoragePolicyUsable(dbConfig.StoragePolicy); err != nil {
		reason := fmt.Sprintf("storage not usable: %v", err)
		if failErr := backupJobRepo.FailBackupJob(ctx, job.ID, reason); failErr != nil {
			return fmt.Errorf("fail backup job %s after unusable storage: %w", job.ID, failErr)
		}
		logger.Warn(fmt.Sprintf("failed backup job %s: %s", job.ID, reason))
		return nil
	}

	blobStoreClient, err := blobstore.NewBlobStoreClient(dbConfig.StoragePolicy, logger)
	if err != nil {
		reason := fmt.Sprintf("create blob store: %v", err)
		if failErr := backupJobRepo.FailBackupJob(ctx, job.ID, reason); failErr != nil {
			return fmt.Errorf("fail backup job %s after blob store error: %w", job.ID, failErr)
		}
		logger.Warn(fmt.Sprintf("failed backup job %s: %s", job.ID, reason))
		return nil
	}

	e, err := exporter.NewExporterWithWindow(eventRepo, eventDeliveryRepo, dbConfig, attemptsRepo, job.HourStart, job.HourEnd, logger)
	if err != nil {
		reason := fmt.Sprintf("create exporter: %v", err)
		if failErr := backupJobRepo.FailBackupJob(ctx, job.ID, reason); failErr != nil {
			return fmt.Errorf("fail backup job %s after exporter error: %w", job.ID, failErr)
		}
		logger.Warn(fmt.Sprintf("failed backup job %s: %s", job.ID, reason))
		return nil
	}

	exportCtx, cancel := context.WithTimeout(ctx, backupExportDeadline)
	defer cancel()

	result, err := e.StreamExport(exportCtx, blobStoreClient)
	if err != nil {
		reason := fmt.Sprintf("stream export: %v", err)
		if failErr := backupJobRepo.FailBackupJob(ctx, job.ID, reason); failErr != nil {
			return fmt.Errorf("fail backup job %s after stream error: %w", job.ID, failErr)
		}
		logger.Warn(fmt.Sprintf("failed backup job %s: %s", job.ID, reason))
		return nil
	}

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

// ManualBackup runs a one-time backup with an explicit time window.
// It always uses the cron-based Exporter, never CDC, regardless of config.
// Shares the instance export mutex with ProcessBackupJob so a trigger cannot
// overlap a scheduled export and starve the DB pool.
func ManualBackup(
	configRepo datastore.ConfigurationRepository,
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	attemptsRepo datastore.DeliveryAttemptsRepository,
	locker JobLocker,
	logger log.Logger,
) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		// Do not skipIfLockBusy: TriggerBackup already returned 202, so a silent
		// skip would report success with no export. Return ErrLockBusy so asynq retries.
		return locker.WithLock(ctx, backupExportMutexName, backupExportMaxRuntime, func(ctx context.Context) error {
			return manualBackup(ctx, t, configRepo, eventRepo, eventDeliveryRepo, attemptsRepo, logger)
		})
	}
}

func manualBackup(
	ctx context.Context,
	t *asynq.Task,
	configRepo datastore.ConfigurationRepository,
	eventRepo datastore.EventRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	attemptsRepo datastore.DeliveryAttemptsRepository,
	logger log.Logger,
) error {
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

	if !dbConfig.GetWebhookArchivingConfig().Enabled {
		return fmt.Errorf("webhook archiving is not enabled")
	}

	if err := blobstore.StoragePolicyUsable(dbConfig.StoragePolicy); err != nil {
		return fmt.Errorf("storage not usable for archive export: %w", err)
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

	exportCtx, cancel := context.WithTimeout(ctx, backupExportDeadline)
	defer cancel()

	result, err := exp.StreamExport(exportCtx, store)
	if err != nil {
		return fmt.Errorf("stream export: %w", err)
	}

	for table, r := range result {
		logger.Info(fmt.Sprintf("manual backup: %s — %d records → %s", table, r.NumDocs, r.ExportFile))
	}

	return nil
}

func generateAgentID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}
