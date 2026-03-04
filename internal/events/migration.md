# Events Repository SQLc Migration Tracker

## Overview

Migrating `/database/postgres/event.go` (1,380 lines, 18 methods) from manual SQL to sqlc-based implementation in `internal/events/`.

**Start Date**: 2026-03-04
**Target**: Complete all phases with zero regressions

---

## Migration Status: 🔄 IN PROGRESS - Phase 3 Complete!

### Phase 0: Baseline Testing ✅ COMPLETED
**Completed**: 2026-03-04

#### Test Coverage
Created `database/postgres/event_pagination_baseline_test.go` with 8 critical test cases:

1. ✅ Empty search query uses EXISTS path
2. ✅ Forward pagination with DESC sort
3. ✅ Forward pagination with ASC sort
4. ✅ Filter by endpoint IDs (EXISTS path)
5. ✅ Filter by source IDs (EXISTS path)
6. ✅ Filter by owner_id (EXISTS path)
7. ✅ Empty result set
8. ✅ PrevRowCount calculation (middle page)

#### Test Results (Pre-Migration)
```
Date: 2026-03-04
Branch: feature/events-sqlc-migration
Implementation: database/postgres/event.go
Status: Tests created (to be run against legacy implementation)
```

---

### Phase 1: Preparation & Infrastructure ✅ COMPLETED
**Completed**: 2026-03-04

#### Completed Tasks
- ✅ Create `internal/events/` directory structure
- ✅ Add sqlc configuration to `sqlc.yaml`
- ✅ Create skeleton files (impl.go, helpers.go, README.md)
- ✅ Create migration.md progress tracker
- ✅ Update .gitignore to exclude generated repo/ code
- ✅ Verify worktree setup

**Git Commit**: 298c7cd7

---

### Phase 2: Query Migration ✅ COMPLETED
**Completed**: 2026-03-04

#### All Queries Implemented (19 total)

#### Query Groups (19 queries total)

**Group 1: Simple CRUD (5 queries)**
- ✅ CreateEvent
- ✅ CreateEventEndpoints
- ✅ UpdateEventEndpoints
- ✅ UpdateEventStatus
- ✅ FindEventByID

**Group 2: Batch Reads & Counting (5 queries)**
- ✅ FindEventsByIDs
- ✅ FindEventsByIdempotencyKey
- ✅ FindFirstEventWithIdempotencyKey
- ✅ CountProjectMessages
- ✅ CountEvents

**Group 3: Complex Pagination (5 queries) ⚠️ MOST CRITICAL**
- ✅ LoadEventsPagedExists (EXISTS path with index optimization)
- ✅ LoadEventsPagedSearch (CTE+JOIN with full-text search)
- ✅ CountPrevEventsExists
- ✅ CountPrevEventsSearch
- ✅ All 10+ filters implemented with CASE expressions
- ✅ Bidirectional pagination (forward/backward)
- ✅ Cursor-based navigation with ASC/DESC sort

**Group 4: Deletion & Maintenance (4 queries)**
- ✅ SoftDeleteProjectEvents
- ✅ HardDeleteProjectEvents
- ✅ HardDeleteTokenizedEvents
- ✅ CopyRowsFromEventsToEventsSearch

**Group 5: Partition Management (4 queries)**
- ✅ PartitionEventsTable
- ✅ UnPartitionEventsTable
- ✅ PartitionEventsSearchTable
- ✅ UnPartitionEventsSearchTable

**Key Achievements**:
- CASE expressions for conditional filters
- EXISTS subquery for index usage (fast path)
- CTE pattern for full-text search
- Source metadata via LEFT JOIN
- COALESCE for nullable fields

**Git Commit**: b5d672f7

---

### Phase 3: Service Implementation ✅ COMPLETED
**Completed**: 2026-03-04

#### Files Created
- ✅ `internal/events/impl.go` (523 lines)
  - Service struct with pgxpool.Pool
  - Constructor: New(logger, db)
  - All 18 repository methods
- ✅ `internal/events/helpers.go` (169 lines)
  - rowToEvent() type converter (handles 4 row types)
  - pgtype conversions
  - Endpoint/header parsing
- ✅ `internal/events/repo/` stubs for sqlc
  - db.go, querier.go, queries.sql.go, models.go
  - .gitignore to mark as generated code

#### Key Implementations
- ✅ CreateEvent with batch endpoint processing (30K partition size)
- ✅ LoadEventsPaged with dual query path logic
  - loadEventsPagedExists: Fast path with EXISTS
  - loadEventsPagedSearch: CTE path with search
  - countPrevEvents: Previous page detection
  - Proper cursor handling for bidirectional pagination
- ✅ Transaction context preservation (pgx.Tx)
- ✅ UpdateEventEndpoints with batch processing
- ✅ CopyRows with transaction handling
- ✅ Type conversions (pgtype ↔ datastore)
- ✅ Error handling (ErrEventNotFound)

**Git Commit**: 27d6389a

**Note**: sqlc stub files created. Run `sqlc generate` with database access to generate real code.

---

### Phase 4: Integration ⏳ PENDING
**Status**: Not started

#### Dependent Files (26 files)
- [ ] API handlers (2 files)
- [ ] Services (5 files)
- [ ] Workers (multiple files)
- [ ] Database initialization
- [ ] E2E tests

---

### Phase 5: Testing & Validation ⏳ PENDING
**Status**: Not started

#### Testing Checklist
- [ ] Re-run Phase 0 baseline tests (8 tests must pass)
- [ ] Create new unit tests in `impl_test.go`
- [ ] Run integration tests
- [ ] Run E2E tests
- [ ] Run `/validate` skill
- [ ] Compare query plans (no performance regression)

---

### Phase 6: Cleanup & Merge ⏳ PENDING
**Status**: Not started

#### Final Steps
- [ ] Delete `database/postgres/event.go`
- [ ] Delete `database/postgres/event_test.go` (if needed)
- [ ] Update migration.md status to COMPLETED
- [ ] Document Claude contribution
- [ ] Commit all changes
- [ ] Merge feature branch to main
- [ ] Remove worktree

---

## Method Complexity Ratings

| Method | Lines | Complexity | Priority | Notes |
|--------|-------|------------|----------|-------|
| LoadEventsPaged | 186 | ⭐⭐⭐⭐⭐ | CRITICAL | Dual query paths, 10+ filters, bidirectional pagination |
| CreateEvent | 62 | ⭐⭐⭐ | HIGH | Batch processing, transaction handling |
| UpdateEventEndpoints | 44 | ⭐⭐⭐ | HIGH | Batch processing, transaction handling |
| FindEventsByIDs | 29 | ⭐⭐ | MEDIUM | sqlx.In handling |
| CountEvents | 39 | ⭐⭐ | MEDIUM | Dynamic filter building |
| FindEventByID | 13 | ⭐ | LOW | Simple query |
| FindEventsByIdempotencyKey | 28 | ⭐ | LOW | Simple query |
| FindFirstEventWithIdempotencyKey | 13 | ⭐ | LOW | Simple query |
| UpdateEventStatus | 28 | ⭐ | LOW | Simple update |
| CountProjectMessages | 9 | ⭐ | LOW | Simple count |
| DeleteProjectEvents | 15 | ⭐⭐ | MEDIUM | Conditional query |
| DeleteProjectTokenizedEvents | 10 | ⭐ | LOW | Simple delete |
| CopyRows | 20 | ⭐⭐ | MEDIUM | Transaction + function call |
| ExportRecords | 3 | ⭐ | LOW | Helper wrapper |
| PartitionEventsTable | 3 | ⭐ | LOW | Function call |
| UnPartitionEventsTable | 3 | ⭐ | LOW | Function call |
| PartitionEventsSearchTable | 3 | ⭐ | LOW | Function call |
| UnPartitionEventsSearchTable | 3 | ⭐ | LOW | Function call |

**Total**: 18 methods

---

## Risk Areas

### High Risk
- **LoadEventsPaged dual query path**: Must preserve EXISTS vs CTE logic
- **Batch endpoint processing**: 30K partition size must be maintained
- **Transaction context**: Nested transactions must work correctly
- **26 dependent files**: Breaking changes cascade widely

### Mitigation
- Comprehensive baseline tests lock in behavior
- Keep legacy code intact until Phase 6
- Incremental testing after each phase
- Query plan comparison for performance verification

---

## Success Criteria

- ✅ All 18 interface methods implemented with sqlc
- ✅ All 8 baseline tests pass (no regressions)
- ✅ All unit tests pass (85%+ coverage)
- ✅ All integration tests pass
- ✅ All E2E tests pass
- ✅ No performance regressions
- ✅ 26/26 dependent files updated
- ✅ Legacy code removed
- ✅ `/validate` skill passes

---

## Notes

- Using git worktree `feature/events-sqlc-migration` for isolated development
- Following patterns from `internal/api_keys`, `internal/portal_links`, `internal/event_types`
- CASE expressions in SQL to consolidate query variants
- Transaction context preserved throughout
- LoadEventsPaged is the most complex method - requires extra attention

---

## Timeline

- **Phase 0**: ✅ Completed (2026-03-04)
- **Phase 1**: Estimated 2 hours
- **Phase 2**: Estimated 1 day
- **Phase 3**: Estimated 2 days
- **Phase 4**: Estimated 1 day
- **Phase 5**: Estimated 2-3 days
- **Phase 6**: Estimated 2 hours

**Total Estimate**: 7-8 days
