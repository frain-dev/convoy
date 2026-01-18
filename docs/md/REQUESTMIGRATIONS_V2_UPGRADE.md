# Design Document: Upgrade requestmigrations to v2 in Convoy

**Author:** Subomi Oluwalana  
**Date:** January 2026  
**Status:** Draft

---

## 1. Overview

This document outlines the plan to upgrade Convoy from `requestmigrations` v0.4.0 to v2. The v2 release introduces a **type-based migration system** that replaces the handler-based approach, offering better code reuse, automatic nested type handling, and simplified migration logic.

---

## 2. Motivation

### Current Pain Points (v0.4.0)
- **Duplication**: Each handler returning an `Endpoint` needs its own migration (`CreateEndpointResponseMigration`, `GetEndpointResponseMigration`, `UpdateEndpointResponseMigration`)
- **Boilerplate**: Migrations require manual JSON marshal/unmarshal and maintaining old struct definitions
- **No nested type support**: If a nested type changes, every parent migration must handle it manually

### Benefits of v2
- **Define once, apply everywhere**: Register migration for `models.EndpointResponse`, automatically applied in all handlers returning that type
- **No old struct definitions**: Migrations work on `map[string]interface{}` directly
- **Automatic nesting**: Library traverses type graph and applies migrations to nested types
- **Cleaner API**: `rm.WithUserVersion(r).Marshal(&data)` replaces manual transformation chains

---

## 3. Current State Analysis

### Files Affected
| File | Usage |
|------|-------|
| `api/api.go` | Creates `RequestMigration` instance, registers migrations |
| `api/migrations.go` | Defines `MigrationStore` map |
| `api/handlers/handlers.go` | Holds `*requestmigrations.RequestMigration` reference |
| `api/migrations/v20240101/*` | 6 migrations with old struct definitions |
| `api/migrations/v20240401/*` | 4 migrations with old struct definitions |
| `api/migrations/v20251124/*` | 2 migrations |

### Current Migration Count
- **v20240101**: `CreateEndpointRequest`, `CreateEndpointResponse`, `GetEndpointResponse`, `GetEndpointsResponse`, `UpdateEndpointRequest`, `UpdateEndpointResponse`
- **v20240401**: `CreateEndpointResponse`, `GetEndpointResponse`, `GetEndpointsResponse`, `UpdateEndpointResponse`
- **v20251124**: `CreatePortalLinkRequest`, `UpdatePortalLinkRequest`

### Current Interface
```go
// v0.4.0 interface
type Migration interface {
    Migrate(b []byte, h http.Header) ([]byte, http.Header, error)
}
```

---

## 4. Target State (v2)

### New Interface
```go
// v2 interface
type TypeMigration interface {
    MigrateForward(data any) (any, error)   // old → new (for requests)
    MigrateBackward(data any) (any, error)  // new → old (for responses)
}
```

### New Registration Pattern
```go
import rms "github.com/subomi/requestmigrations/v2"

// Register by TYPE, not by handler
rms.Register[models.CreateEndpoint](rm, "2024-01-01", &migrations.CreateEndpointV20240101{})
rms.Register[models.EndpointResponse](rm, "2024-01-01", &migrations.EndpointResponseV20240101{})
rms.Register[models.EndpointResponse](rm, "2024-04-01", &migrations.EndpointResponseV20240401{})
rms.Register[datastore.CreatePortalLinkRequest](rm, "2025-11-24", &migrations.PortalLinkRequestV20251124{})
```

### New Handler Usage
```go
func (h *Handler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    
    var req models.CreateEndpoint
    // Automatic forward migration for old clients
    if err := h.RM.WithUserVersion(r).Unmarshal(body, &req); err != nil {
        // handle error
    }
    
    // ... business logic ...
    
    // Automatic backward migration for old clients
    response, _ := h.RM.WithUserVersion(r).Marshal(&endpointResponse)
    w.Write(response)
}
```

---

## 5. Migration Consolidation

v2 allows consolidating handler-specific migrations into type-based migrations:

| v0.4.0 Migrations | v2 Type | v2 Migration |
|-------------------|---------|--------------|
| `CreateEndpointRequestMigration` | `models.CreateEndpoint` | `CreateEndpointMigration` |
| `UpdateEndpointRequestMigration` | `models.UpdateEndpoint` | `UpdateEndpointMigration` |
| `CreateEndpointResponseMigration`, `GetEndpointResponseMigration`, `UpdateEndpointResponseMigration` | `models.EndpointResponse` | `EndpointResponseMigration` |
| `GetEndpointsResponseMigration` | `[]models.EndpointResponse` or wrapper type | `EndpointsResponseMigration` |
| `CreatePortalLinkRequestMigration`, `UpdatePortalLinkRequestMigration` | `datastore.CreatePortalLinkRequest` | `PortalLinkRequestMigration` |

**Estimated reduction**: 12 migration structs → ~6 migration structs

---

## 6. Implementation Plan

### Phase 1: Setup
1. Update `go.mod` to use `github.com/subomi/requestmigrations/v2`
2. Update import paths in affected files
3. Update `RequestMigrationOptions` initialization (API is similar)

### Phase 2: Rewrite Migrations
For each existing migration:
1. Create new migration struct implementing `TypeMigration`
2. Convert `Migrate(b []byte, h http.Header)` logic to:
   - `MigrateForward(data any)` for request migrations
   - `MigrateBackward(data any)` for response migrations
3. Remove old struct definitions (work directly on `map[string]interface{}`)

### Phase 3: Update Registration
1. Remove `MigrationStore` map from `api/migrations.go`
2. Add `rms.Register[T]()` calls for each type/version combination
3. Update `api/api.go` initialization

### Phase 4: Update Handlers
1. Identify handlers that need versioned request/response handling
2. Replace current JSON handling with `rm.WithUserVersion(r).Unmarshal()` / `Marshal()`
3. Remove any manual migration invocations

### Phase 5: Cleanup
1. Delete unused old struct definitions
2. Remove legacy migration files if fully replaced
3. Update tests

---

## 7. Example Migration Rewrite

### Before (v0.4.0)
```go
type oldCreateEndpoint struct {
    URL                string `json:"url"`
    AdvancedSignatures *bool  `json:"advanced_signatures"`
    HttpTimeout        string `json:"http_timeout"`  // was string
    // ...
}

type CreateEndpointRequestMigration struct{}

func (c *CreateEndpointRequestMigration) Migrate(b []byte, h http.Header) ([]byte, http.Header, error) {
    var payload oldCreateEndpoint
    if err := json.Unmarshal(b, &payload); err != nil {
        return nil, nil, err
    }
    
    var endpoint models.CreateEndpoint
    // ... transform old → new ...
    
    if payload.AdvancedSignatures == nil {
        val := false
        endpoint.AdvancedSignatures = &val
    }
    
    b, err := json.Marshal(endpoint)
    return b, h, err
}
```

### After (v2)
```go
type CreateEndpointV20240101 struct{}

func (m *CreateEndpointV20240101) MigrateForward(data any) (any, error) {
    d := data.(map[string]interface{})
    
    // Set default for advanced_signatures if not present
    if _, ok := d["advanced_signatures"]; !ok {
        d["advanced_signatures"] = false
    }
    
    // Convert http_timeout from string to uint64 if needed
    if timeout, ok := d["http_timeout"].(string); ok {
        d["http_timeout"] = parseTimeoutToMs(timeout)
    }
    
    return d, nil
}

func (m *CreateEndpointV20240101) MigrateBackward(data any) (any, error) {
    d := data.(map[string]interface{})
    
    // Convert http_timeout back to string for old clients
    if timeout, ok := d["http_timeout"].(float64); ok {
        d["http_timeout"] = fmt.Sprintf("%dms", int(timeout))
    }
    
    return d, nil
}
```

---

## 8. Handler Integration Approach

### Direct Handler Modification (Chosen Approach)

Handlers will call `rm.WithUserVersion(r).Unmarshal()` and `rm.Marshal()` directly. A middleware-based approach is not feasible because v2's `Marshal`/`Unmarshal` methods require knowing the target type at call time (via `reflect.TypeOf(v)`), which varies per endpoint.

### Handlers Requiring Modification

**Request Migrations (Unmarshal)**
- `CreateEndpoint` - transforms incoming `models.CreateEndpoint`
- `UpdateEndpoint` - transforms incoming `models.UpdateEndpoint`
- `CreatePortalLink` - transforms incoming `datastore.CreatePortalLinkRequest`
- `UpdatePortalLink` - transforms incoming request

**Response Migrations (Marshal)**
- `CreateEndpoint`, `GetEndpoint`, `UpdateEndpoint` - transforms outgoing `models.EndpointResponse`
- `GetEndpoints` - transforms outgoing `[]models.EndpointResponse`

---

## 9. Testing Strategy

### Reuse Existing Tests

Existing migration tests (e.g., `create_endpoint_migration_test.go`) can be adapted for v2:
- **Keep**: Test case names, input values, expected outputs, assertion logic
- **Update**: Change input from struct to `map[string]interface{}`, replace `Migrate(body, header)` with `MigrateForward(data)`/`MigrateBackward(data)`, remove `http.Header` handling

```go
// Before (v0.4.0)
migration := CreateEndpointRequestMigration{}
res, _, err := migration.Migrate(body, header)

// After (v2)
migration := CreateEndpointV20240101{}
result, err := migration.MigrateForward(payload)
data := result.(map[string]interface{})
```

### Success Criteria

All existing migration tests pass after updating to v2 API.

---

## 10. Rollback Plan

If issues arise:
1. Revert `go.mod` to v0.4.0
2. Restore original migration files from git
3. No database changes required (this is API-layer only)

---

## 11. Timeline

| Phase | Estimated Effort |
|-------|------------------|
| Phase 1: Setup | 1 hour |
| Phase 2: Rewrite Migrations | 4-6 hours |
| Phase 3: Update Registration | 1 hour |
| Phase 4: Update Handlers | 2-3 hours |
| Phase 5: Cleanup & Testing | 2-3 hours |
| **Total** | **~1-2 days** |

---

## 12. References

- [requestmigrations v2 Documentation](https://pkg.go.dev/github.com/subomi/requestmigrations/v2)
- [requestmigrations v2 Design Doc](https://github.com/subomi/requestmigrations/blob/main/docs/DESIGN.md)
- [Stripe API Versioning](https://stripe.com/blog/api-versioning)
