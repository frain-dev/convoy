# Queue Metrics Async Snapshot Design

This document proposes an asynchronous snapshot architecture for queue metrics:

- Aggregate queue metrics from PostgreSQL on a fixed interval in a background job.
- Persist the aggregated result to Redis as an atomic snapshot.
- Serve Prometheus scrapes from Redis for low-latency, low-DB-impact reads.

## Motivation

Current queue metrics include rich domain labels and are derived from PostgreSQL/materialized views. This gives good observability but creates pressure on the database during scrape bursts and expensive query windows.

The async snapshot pattern decouples:

- **compute path** (periodic, controlled, backoff-aware)
- **read path** (fast, constant, scrape-friendly)

## Goals

- Significantly reduce PostgreSQL load caused by scrape-time metrics collection.
- Keep queue metric names and labels stable for existing dashboards and alerts.
- Ensure scrape latency is predictable and low.
- Provide explicit freshness/staleness visibility.

## Non-goals

- Replacing all metrics in one iteration.
- Rebuilding queue metrics solely from Asynq task scans.
- Introducing high-complexity streaming infrastructure for initial rollout.

## Proposed Architecture

### 1) Background Aggregator

A goroutine (or scheduled worker task) runs every `refresh_interval`:

1. query PostgreSQL/materialized views for queue metrics
2. transform rows into normalized metric series
3. write into Redis under a versioned snapshot key
4. atomically switch `current` pointer to the new version

### 2) Redis Snapshot Store

Redis stores:

- snapshot payload (all metric series + label values + numeric values)
- metadata (generation timestamp, source duration, status)
- version pointer (`current`)

Prometheus collector reads only the current snapshot.

### 3) Prometheus Collector (Read Path)

Collector flow:

1. read `current` snapshot version key
2. fetch snapshot payload and metadata
3. emit metrics from snapshot
4. emit snapshot health metrics (age, refresh errors, last refresh duration)

No PostgreSQL call on scrape path.

## Data Model in Redis

Use versioned keys to avoid partial reads.

- `convoy:metrics:queue:snapshot:current` -> `v12345`
- `convoy:metrics:queue:snapshot:v12345:data`
- `convoy:metrics:queue:snapshot:v12345:meta`

Suggested metadata fields:

- `generated_at_unix`
- `source` (`postgres_mv`)
- `aggregation_duration_ms`
- `rows_total`
- `status` (`ok` or `error`)
- `error_message` (optional, truncated)

Optionally set TTL on old versions and keep last `N` snapshots for debugging.

## Atomicity and Consistency

Avoid partial snapshot exposure:

1. write new payload to `:vNEW:*`
2. validate payload completeness
3. atomically update `:current` to `vNEW` (single `SET` after successful write)
4. garbage-collect old versions asynchronously

Scrapes always read a fully-formed snapshot version.

## Refresh Cadence and Freshness

Choose an initial interval (for example, 15s to 60s) based on DB capacity and metric freshness requirements.

Expose freshness metric:

- `convoy_queue_metrics_snapshot_age_seconds`

This lets alerts detect stale snapshots independent of business metric values.

## Failure Behavior

If refresh fails:

- keep serving last good snapshot
- increment refresh error counter
- expose failure timestamp and age
- retry with exponential backoff and jitter (bounded)

Do not publish an incomplete snapshot.

## Concurrency and Leadership

If multiple service instances can run the refresher:

- elect a single writer via Redis lock/lease (`SET NX PX`)
- include lock owner identity in metadata
- refresh lock heartbeat while job is running

Readers remain stateless and horizontally scalable.

## Metric Compatibility

Preserve existing queue metric names/label keys where possible:

- `convoy_event_queue_total`
- `convoy_event_queue_backlog_seconds`
- `convoy_event_delivery_queue_total`
- `convoy_event_delivery_attempts_total`
- `convoy_event_delivery_queue_backlog_seconds`

Add snapshot operational metrics:

- `convoy_queue_metrics_snapshot_age_seconds` (gauge)
- `convoy_queue_metrics_refresh_duration_seconds` (gauge or histogram)
- `convoy_queue_metrics_refresh_total` (counter by status)
- `convoy_queue_metrics_snapshot_last_success_timestamp_seconds` (gauge)

## Performance Expectations

Compared to scrape-time DB querying:

- scrape latency becomes near-constant
- DB load shifts from bursty scrape traffic to predictable interval jobs
- worst-case scrape failures due to DB slowness are reduced

Tradeoff: metrics are eventually consistent by one refresh interval.

## Operational Controls

Add config options:

- `metrics.queue_snapshot.refresh_interval`
- `metrics.queue_snapshot.lock_ttl`
- `metrics.queue_snapshot.max_staleness`
- `metrics.queue_snapshot.redis_prefix`
- `metrics.queue_snapshot.keep_versions`

## Rollout Plan

### Phase 1: Build in Shadow Mode

- Run background snapshot generation.
- Validate snapshot correctness with integration tests and controlled load tests before rollout.

### Phase 2: Enable Read-From-Snapshot

- Switch collector to Redis snapshot reads.
- Monitor freshness, error rate, and metric parity.

### Phase 3: Decommission DB Scrape Path

- Remove scrape-time DB aggregation path entirely.
- Keep refresh job as the only producer of queue metrics data.
- Update runbooks and dashboards for snapshot health metrics.

## Validation and Testing

### Unit Tests

- snapshot encoding/decoding
- atomic pointer swap behavior
- staleness calculations
- partial-write rejection

### Integration Tests

- PostgreSQL -> Redis snapshot pipeline correctness
- collector emits expected series from snapshot
- refresh failure preserves last-good snapshot
- multi-instance lock behavior

### Load Tests

- scrape latency under high concurrent scrape traffic
- DB load before/after comparison
- refresh duration under production-like data volume

## Risks and Mitigations

- **Stale metrics due to repeated refresh failure**
  - Mitigation: staleness alerts + last-good serving + bounded retries.
- **Split-brain writers**
  - Mitigation: lock lease + owner metadata + idempotent version naming.
- **Redis memory growth**
  - Mitigation: retain only last `N` versions and enforce TTL.

## Open Questions

- Should refresher run in API process, worker process, or dedicated lightweight metrics job?
- What is the acceptable freshness SLO per metric family?
- Should backlog and attempts metrics use different refresh intervals?
- Do we need per-org/project snapshot partitioning for large scale?

## Recommendation

Adopt this async snapshot architecture as the next step to reduce DB pressure with minimal disruption:

- fastest path to low-latency scrapes
- keeps existing metric contracts largely intact
- leaves room for future middleware/write-time instrumentation where needed

