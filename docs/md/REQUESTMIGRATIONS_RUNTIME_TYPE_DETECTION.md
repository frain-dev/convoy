# Feature Request: Runtime Type Detection for Dynamic Fields

## Problem

The current `buildTypeGraphRecursive` function uses static type reflection (`field.Type`) to build the migration type graph. This works for statically-typed struct fields but fails for:

1. **`interface{}` fields** - The compile-time type is `interface{}`, so nested migrations aren't discovered
2. **`json.RawMessage` fields** - Treated as `[]byte`, not as serialized typed data

## Example Failing Case

```go
type PagedResponse struct {
    Content    interface{}  `json:"content"`
    Pagination *Pagination  `json:"pagination"`
}

type ServerResponse struct {
    Status  bool            `json:"status"`
    Message string          `json:"message"`
    Data    json.RawMessage `json:"data"`
}

// User registers migration for EndpointResponse
Register[EndpointResponse](rm, "2024-01-01", &EndpointMigration{})

// This should apply EndpointResponse migrations to Content, but doesn't:
rm.Marshal(&PagedResponse{Content: []EndpointResponse{...}})

// This should apply migrations to Data, but doesn't:
rm.Marshal(&ServerResponse{Data: marshaledEndpointBytes})
```

## Expected Behavior

When marshaling/unmarshaling, the library should:

1. For `interface{}` fields: inspect the **runtime value** using `reflect.TypeOf(actualValue)` and check if any registered migrations apply
2. For `json.RawMessage` fields: optionally parse and attempt to match against registered type migrations (this may need a hint mechanism)

## Suggested Implementation

### Phase 1: Runtime Type Detection for interface{} Fields

In `migrateForward` and `migrateBackward`, when processing `map[string]interface{}`:

```go
case map[string]interface{}:
    for fieldName, fieldData := range v {
        // Current: only process fields in graph.Fields
        // New: also check runtime type of fieldData against registered migrations
        
        if fieldGraph, ok := graph.Fields[fieldName]; ok {
            // existing logic
        } else if fieldData != nil {
            // NEW: Check if runtime type has registered migrations
            runtimeType := reflect.TypeOf(fieldData)
            if runtimeGraph := rm.buildTypeGraphForRuntimeValue(runtimeType, fieldData, userVersion); runtimeGraph.HasMigrations() {
                // Apply migrations to this dynamically-typed field
            }
        }
    }
```

### Phase 2: Handle Slices Inside interface{}

```go
// If fieldData is []interface{}, check if elements match a registered type
// by examining the first element's structure against registered type schemas
func (rm *RequestMigration) detectSliceElementType(slice []interface{}, userVersion *Version) *TypeGraph {
    if len(slice) == 0 {
        return nil
    }
    
    // Check first element against all registered types
    firstElem := slice[0]
    if m, ok := firstElem.(map[string]interface{}); ok {
        // Compare field names against registered struct types
        for registeredType := range rm.migrations {
            if rm.structMatchesMap(registeredType, m) {
                return rm.buildTypeGraph(registeredType, userVersion)
            }
        }
    }
    return nil
}
```

### Phase 3: json.RawMessage Support (Optional)

For `json.RawMessage`, structural matching is more complex since it's already serialized:

```go
// Option A: Type hint in field tag
type ServerResponse struct {
    Data json.RawMessage `json:"data" migrate:"EndpointResponse"`
}

// Option B: Explicit migration registration for the wrapper
Register[ServerResponse](rm, "2024-01-01", &ServerResponseMigration{
    DataType: reflect.TypeOf(EndpointResponse{}),
})

// Option C: Parse and attempt structural matching (expensive)
func (rm *RequestMigration) migrateRawMessage(raw json.RawMessage, userVersion *Version) (json.RawMessage, error) {
    var parsed interface{}
    json.Unmarshal(raw, &parsed)
    // Attempt to match against registered types...
}
```

## Considerations

### Performance
- Runtime type detection adds overhead
- Consider caching runtime type graphs keyed by `(reflect.Type, version)`
- Structural matching against all registered types is O(n) per field

### Ambiguity
- Multiple registered types might match the same JSON structure
- Mitigation options:
  - Require exact field match (all fields must be present)
  - Use type hints/annotations
  - First-match wins with deterministic ordering

### json.RawMessage Complexity
- Already serialized, so we can't use `reflect.TypeOf`
- Options: type hints, structural matching, or explicit wrapper migrations
- May warrant a separate opt-in mechanism

### Backward Compatibility
- This should be additive - existing behavior for statically-typed fields remains unchanged
- New behavior only activates for `interface{}` fields with runtime values

## Test Cases

```go
func TestInterfaceFieldMigration(t *testing.T) {
    rm, _ := NewRequestMigration(&RequestMigrationOptions{
        CurrentVersion: "2024-06-01",
        VersionFormat:  DateFormat,
    })
    
    // Register migration for nested type
    Register[EndpointResponse](rm, "2024-01-01", &EndpointMigration{
        // MigrateBackward: converts name -> title
    })
    
    // Create request with old version
    req := httptest.NewRequest("GET", "/", nil)
    req.Header.Set("X-API-Version", "2023-01-01")
    
    // Marshal wrapper with interface{} field containing registered type
    wrapper := &PagedResponse{
        Content: []EndpointResponse{{Name: "test-endpoint"}},
    }
    result, err := rm.WithUserVersion(req).Marshal(wrapper)
    require.NoError(t, err)
    
    // Assert: EndpointResponse migrations were applied to Content
    var parsed map[string]interface{}
    json.Unmarshal(result, &parsed)
    
    content := parsed["content"].([]interface{})
    firstItem := content[0].(map[string]interface{})
    
    // Should have "title" (old field name), not "name" (new field name)
    assert.Equal(t, "test-endpoint", firstItem["title"])
    assert.Nil(t, firstItem["name"])
}

func TestNestedInterfaceSliceMigration(t *testing.T) {
    // Test that []EndpointResponse inside interface{} gets migrated
}

func TestInterfaceFieldWithNilValue(t *testing.T) {
    // Test that nil interface{} fields don't cause panics
}

func TestInterfaceFieldWithUnregisteredType(t *testing.T) {
    // Test that interface{} containing unregistered types passes through unchanged
}

func TestRawMessageFieldMigration(t *testing.T) {
    // Test for json.RawMessage fields (if implementing Phase 3)
}
```

## Migration Path

1. Implement Phase 1 (interface{} runtime detection) first - highest value, moderate complexity
2. Phase 2 (slice detection) builds on Phase 1
3. Phase 3 (json.RawMessage) is optional and can be deferred based on demand

## References

- Current implementation: `buildTypeGraphRecursive` in `requestmigrations.go`
- Related: `migrateForward` and `migrateBackward` functions
