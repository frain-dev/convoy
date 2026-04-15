# Jobs & Feature Flags: SQLc Migration Progress

## Overview
Migration of the last remaining `database/postgres/` repository implementations to sqlc-based modules under `internal/`.

## Modules Migrated

### 1. Jobs (`internal/jobs/`)
- **Interface**: `datastore.JobRepository` (9 methods)
- **Old implementation**: `database/postgres/job.go` (deleted)
- **New files**: `internal/jobs/impl.go`, `internal/jobs/queries.sql`, `internal/jobs/repo/` (generated)
- **Call site**: `internal/dataplane/worker.go` updated from `postgres.NewJobRepo()` to `jobs.New()`

### 2. Feature Flags (`internal/feature_flags/`)
- **Interfaces**: `fflag.FeatureFlagFetcher`, `fflag.EarlyAdopterFeatureFetcher`
- **Old implementation**: `database/postgres/feature_flag.go`, `feature_flag_fetcher.go`, `early_adopter_feature_fetcher.go` (all deleted)
- **New files**: `internal/feature_flags/impl.go`, `internal/feature_flags/queries.sql`, `internal/feature_flags/repo/` (generated)
- **Tables**: `convoy.feature_flags`, `convoy.feature_flag_overrides`, `convoy.early_adopter_features`
- **Call sites updated**:
  - `cmd/server/server.go`
  - `internal/dataplane/worker.go`
  - `api/handlers/organisation.go`
  - `cmd/utils/org_feature_flags.go`
  - `api/server_suite_test.go`
  - `api/oss_login_integration_test.go`
  - `api/oauth2_integration_test.go`
  - `e2e/oauth2_e2e_test.go`

## Files Deleted
- `database/postgres/job.go`
- `database/postgres/job_test.go`
- `database/postgres/feature_flag.go`
- `database/postgres/feature_flag_fetcher.go`
- `database/postgres/early_adopter_feature_fetcher.go`

## Remaining in `database/postgres/`
Only infrastructure files:
- `postgres.go` - DB connection pool management
- `postgres_collector.go` - Prometheus metrics
- `postgres_test.go` - Infrastructure tests
- `pkg_logger.go` - Package logger

## Status: Complete
All repository implementations have been migrated to sqlc.
