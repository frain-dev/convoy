# Queue Metrics Improvement Design

This document proposes a simpler, lower-risk way to improve queue metrics collection while reducing PostgreSQL pressure.

## Context

Current queue metrics are collected from two places:

- Redis queue inspector metrics for queue-level state.
- PostgreSQL/materialized view metrics for domain-specific breakdowns (`project`, `source`, `endpoint`, `status`, `http_status_code`).

The PostgreSQL side provides rich business dimensions, but it introduces DB load from periodic aggregation queries and MV refreshes.

## Goals

- Reduce database load for queue metrics collection.
- Preserve existing Prometheus metric names where possible.
- Keep domain-specific visibility for operations and debugging.
- Avoid high-complexity infrastructure additions for first rollout.

## Non-goals

- Full redesign of all ingestion/data-plane metrics.
- Immediate replacement of every DB-derived metric in one release.
- Introduction of operationally heavy components unless needed.

## Metric Inventory and Source Strategy

### Queue Health Metrics (derive from Redis/Asynq)

- `convoy_event_queue_scheduled_total`
- queue size/state/latency-style metrics tied to queue runtime status

These metrics are naturally queue/state centric and should come from Asynq inspector-backed collection.

### Domain Metrics (record at write/process time)

- `convoy_event_queue_total`
- `convoy_event_queue_backlog_seconds`
- `convoy_event_delivery_queue_total`
- `convoy_event_delivery_queue_backlog_seconds`
- `convoy_event_delivery_attempts_total`

These metrics need domain labels and history semantics. They should be emitted/aggregated at enqueue/process time rather than reconstructed from scrape-time queue snapshots.

## Why QueueInfo Alone Is Not Enough

Asynq `QueueInfo` gives aggregated queue snapshots (`pending`, `active`, `retry`, `size`, `latency`, etc). It does not expose per-task domain labels needed for full parity with current business metrics.

Rebuilding domain metrics by scanning tasks at scrape time would be:

- expensive at scale (Redis CPU/network, scraper CPU)
- race-prone while task states change
- weak for historical counter semantics (for example attempts totals)

## Proposed Architecture (Hybrid)

Use a two-lane metrics model:

1. **Lane A: Redis/Asynq Queue Collector**
   - Keep or improve current Redis queue collector.
   - Focus on infra health metrics (queue depth/latency/state).

2. **Lane B: Domain Metrics Recorder**
   - Add explicit instrumentation in workers at event lifecycle points:
     - event created/enqueued
     - delivery queued
     - delivery status transition
     - delivery attempt (status code aware)
   - Persist/update counters in a low-cost store (Redis hash/counters or in-process Prometheus counters with safe durability strategy).

3. **Prometheus Collector Layer**
   - Export both lanes through one registry.
   - Keep metric names stable; only adjust internals/source.

## Where Middleware Strategy Works

Middleware is the preferred first step for domain metrics because it centralizes instrumentation without editing every handler.

### Works Well

- Process-time counters and timers (attempts, success/failure, in-progress, handler latency).
- Domain label extraction from task payload/headers when metadata is present and stable.
- Consistent instrumentation across all task types via one `ServeMux.Use(...)` chain.

### Works Partially

- Queue totals by domain can be approximated from processing signals, but not always equivalent to true enqueue-time backlog state.
- Retry-aware metrics need idempotency safeguards to avoid double counting during reprocessing.

### Does Not Cover Alone

- Enqueue-time metrics (task created/scheduled but not yet processed).
- Backlog-by-domain from pending tasks that have not entered middleware yet.
- Pure queue snapshot metrics (`size`, `latency`, queue state totals), which still belong to queue collectors.

### Metric-by-Metric Fit

- `convoy_event_queue_scheduled_total`: **Queue collector** (not middleware).
- `convoy_event_queue_total`: **Middleware + enqueue hook** for best parity.
- `convoy_event_queue_backlog_seconds`: **Queue collector** (or dedicated backlog index), not middleware alone.
- `convoy_event_delivery_queue_total`: **Middleware + enqueue hook** for best parity.
- `convoy_event_delivery_queue_backlog_seconds`: **Queue collector** (or dedicated backlog index), not middleware alone.
- `convoy_event_delivery_attempts_total`: **Middleware** (strong fit) with counter/idempotency guarantees.

## Data Model Guidance for Domain Metrics

Prefer stable, bounded labels:

- `project`
- `source`
- `endpoint`
- `status`
- `http_status_code`

Avoid unbounded labels (event IDs, payload fragments, dynamic URLs).

## Implementation Plan

### Phase 1: Baseline and Guardrails

- Catalog current metrics and expected label sets.
- Add tests that assert metric names/labels exposed today.

### Phase 2: Queue Collector Cleanup

- Keep queue inspector collector as source of truth for queue-state metrics.
- Ensure naming/help strings are consistent and documented.

### Phase 3: Middleware-first Domain Recording

- Add global Asynq middleware to record attempts, status outcomes, and processing latency.
- Decode bounded domain labels from task metadata in middleware.
- Add minimal enqueue hooks only for metrics middleware cannot observe.
- Add idempotency guards where retries can re-run logic.
- Ensure `*_attempts_total` has true counter semantics.

### Phase 4: Progressive DB Offload

- Shift selected DB-derived metrics to new recorder-backed source.
- Keep DB fallback behind config until confidence is high.

### Phase 5: Verification and Rollout

- Run both paths in parallel (shadow mode) and compare outputs.
- Define tolerances and alert on drift.
- Remove/deprecate DB-heavy paths after stable parity.

## Backward Compatibility

- Keep existing metric names and label keys where possible.
- If a metric semantic must change, publish migration notes and add temporary compatibility metrics.

## Risks and Mitigations

- **Double counting during retries**
  - Mitigation: idempotent update keys and clear lifecycle ownership.
- **Label cardinality growth**
  - Mitigation: strict label allowlist and normalization.
- **Source divergence during migration**
  - Mitigation: shadow comparisons and staged rollout.

## Open Questions

- Should domain counters live in Redis, Postgres-derived cache tables, or solely in Prometheus process memory?
- What freshness/SLA is required for each dashboard and alert?
- Which metrics can tolerate approximate values during migration?

## Success Criteria

- Reduced DB query/MV-refresh load attributable to metrics.
- No loss of required domain observability.
- Stable dashboards and alerts after migration.
- Clear operational runbook for collector and recorder failure modes.

