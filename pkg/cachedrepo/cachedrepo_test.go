package cachedrepo

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- test mocks ---

type mockLogger struct{}

func (m *mockLogger) Error(args ...any) {}

type mockCache struct {
	data   map[string]interface{}
	getErr error
	setErr error
	delErr error

	getCalled    int
	setCalled    int
	deleteCalled int
	setFunc      func(key string, data interface{}) // optional hook
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string]interface{})}
}

func (m *mockCache) Get(_ context.Context, key string, data interface{}) error {
	m.getCalled++
	if m.getErr != nil {
		return m.getErr
	}
	// no-op: data stays zero-valued (simulates cache miss)
	return nil
}

func (m *mockCache) Set(_ context.Context, key string, data interface{}, _ time.Duration) error {
	m.setCalled++
	if m.setFunc != nil {
		m.setFunc(key, data)
	}
	if m.setErr != nil {
		return m.setErr
	}
	m.data[key] = data
	return nil
}

func (m *mockCache) Delete(_ context.Context, key string) error {
	m.deleteCalled++
	if m.delErr != nil {
		return m.delErr
	}
	delete(m.data, key)
	return nil
}

// --- test entity ---

type testEntity struct {
	UID  string
	Name string
}

// --- FetchOne tests ---

func TestFetchOne_CacheMiss(t *testing.T) {
	ca := newMockCache()
	logger := &mockLogger{}
	entity := &testEntity{UID: "123", Name: "test"}

	result, err := FetchOne(context.Background(), ca, logger, "key:123", time.Minute,
		func(e *testEntity) bool { return e.UID != "" },
		func() (*testEntity, error) { return entity, nil })

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UID != "123" {
		t.Fatalf("expected UID 123, got %s", result.UID)
	}
	if ca.setCalled != 1 {
		t.Fatalf("expected Set to be called once, got %d", ca.setCalled)
	}
}

func TestFetchOne_CacheHit(t *testing.T) {
	ca := &hitCache{val: testEntity{UID: "123", Name: "cached"}}
	logger := &mockLogger{}
	fetchCalled := false

	result, err := FetchOne(context.Background(), ca, logger, "key:123", time.Minute,
		func(e *testEntity) bool { return e.UID != "" },
		func() (*testEntity, error) {
			fetchCalled = true
			return nil, errors.New("should not be called")
		})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UID != "123" || result.Name != "cached" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if fetchCalled {
		t.Fatal("fetch should not have been called on cache hit")
	}
}

func TestFetchOne_CacheError_FallsThrough(t *testing.T) {
	ca := newMockCache()
	ca.getErr = errors.New("redis down")
	logger := &mockLogger{}
	entity := &testEntity{UID: "123"}

	result, err := FetchOne(context.Background(), ca, logger, "key:123", time.Minute,
		func(e *testEntity) bool { return e.UID != "" },
		func() (*testEntity, error) { return entity, nil })

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UID != "123" {
		t.Fatalf("expected UID 123, got %s", result.UID)
	}
}

func TestFetchOne_DBError(t *testing.T) {
	ca := newMockCache()
	logger := &mockLogger{}

	result, err := FetchOne(context.Background(), ca, logger, "key:123", time.Minute,
		func(e *testEntity) bool { return e.UID != "" },
		func() (*testEntity, error) { return nil, errors.New("db error") })

	if err == nil {
		t.Fatal("expected error")
	}
	if result != nil {
		t.Fatal("expected nil result on DB error")
	}
	if ca.setCalled != 0 {
		t.Fatal("Set should not be called on DB error")
	}
}

// --- FetchSlice tests ---

func TestFetchSlice_CacheMiss(t *testing.T) {
	ca := newMockCache()
	logger := &mockLogger{}
	items := []testEntity{{UID: "1"}, {UID: "2"}}

	result, err := FetchSlice(context.Background(), ca, logger, "key:list", time.Minute,
		func() ([]testEntity, error) { return items, nil })

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if ca.setCalled != 1 {
		t.Fatalf("expected Set called once, got %d", ca.setCalled)
	}
}

func TestFetchSlice_CacheHit(t *testing.T) {
	ca := &hitCache{val: sliceWrapper[testEntity]{Items: []testEntity{{UID: "cached"}}}}
	logger := &mockLogger{}
	fetchCalled := false

	result, err := FetchSlice(context.Background(), ca, logger, "key:list", time.Minute,
		func() ([]testEntity, error) {
			fetchCalled = true
			return nil, errors.New("should not be called")
		})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].UID != "cached" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if fetchCalled {
		t.Fatal("fetch should not be called on cache hit")
	}
}

func TestFetchSlice_CacheHitEmptySlice(t *testing.T) {
	ca := &hitCache{val: sliceWrapper[testEntity]{Items: []testEntity{}}}
	logger := &mockLogger{}

	result, err := FetchSlice(context.Background(), ca, logger, "key:list", time.Minute,
		func() ([]testEntity, error) { return nil, errors.New("should not be called") })

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(result))
	}
}

// --- FetchWithNotFound tests ---

var errNotFound = errors.New("not found")

func TestFetchWithNotFound_CacheMiss_Found(t *testing.T) {
	ca := newMockCache()
	logger := &mockLogger{}
	entity := &testEntity{UID: "123"}

	result, err := FetchWithNotFound(context.Background(), ca, logger, "key:123", time.Minute,
		func() (*testEntity, error) { return entity, nil },
		func(err error) bool { return errors.Is(err, errNotFound) },
		errNotFound)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UID != "123" {
		t.Fatalf("expected UID 123, got %s", result.UID)
	}
	if ca.setCalled != 1 {
		t.Fatalf("expected Set called once, got %d", ca.setCalled)
	}
}

func TestFetchWithNotFound_CacheMiss_NotFound_Cached(t *testing.T) {
	ca := newMockCache()
	logger := &mockLogger{}

	result, err := FetchWithNotFound(context.Background(), ca, logger, "key:123", time.Minute,
		func() (*testEntity, error) { return nil, errNotFound },
		func(err error) bool { return errors.Is(err, errNotFound) },
		errNotFound)

	if !errors.Is(err, errNotFound) {
		t.Fatalf("expected errNotFound, got %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result")
	}
	if ca.setCalled != 1 {
		t.Fatalf("expected Set called once (caching not-found), got %d", ca.setCalled)
	}
}

func TestFetchWithNotFound_CacheHit_Found(t *testing.T) {
	entity := &testEntity{UID: "123"}
	ca := &hitCache{val: foundWrapper[testEntity]{Value: entity, Found: true}}
	logger := &mockLogger{}

	result, err := FetchWithNotFound(context.Background(), ca, logger, "key:123", time.Minute,
		func() (*testEntity, error) { return nil, errors.New("should not be called") },
		func(err error) bool { return false },
		errNotFound)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UID != "123" {
		t.Fatalf("expected UID 123, got %s", result.UID)
	}
}

func TestFetchWithNotFound_CacheHit_CachedNotFound(t *testing.T) {
	ca := &hitCache{val: foundWrapper[testEntity]{Value: nil, Found: true}}
	logger := &mockLogger{}

	result, err := FetchWithNotFound(context.Background(), ca, logger, "key:123", time.Minute,
		func() (*testEntity, error) { return nil, errors.New("should not be called") },
		func(err error) bool { return false },
		errNotFound)

	if !errors.Is(err, errNotFound) {
		t.Fatalf("expected errNotFound, got %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for cached not-found")
	}
}

// --- Invalidate tests ---

func TestInvalidate(t *testing.T) {
	ca := newMockCache()
	logger := &mockLogger{}

	Invalidate(context.Background(), ca, logger, "key:1", "key:2", "", "key:3")

	if ca.deleteCalled != 3 {
		t.Fatalf("expected 3 Delete calls (skipping empty), got %d", ca.deleteCalled)
	}
}

func TestInvalidate_ErrorLogged(t *testing.T) {
	ca := newMockCache()
	ca.delErr = errors.New("delete failed")
	logger := &mockLogger{}

	// Should not panic or return error
	Invalidate(context.Background(), ca, logger, "key:1")

	if ca.deleteCalled != 1 {
		t.Fatalf("expected 1 Delete call, got %d", ca.deleteCalled)
	}
}

// --- hitCache simulates a cache that returns a populated value on Get ---

type hitCache struct {
	val interface{}
}

func (h *hitCache) Get(_ context.Context, _ string, data interface{}) error {
	// Use type switch to populate the data pointer with the stored value.
	switch d := data.(type) {
	case *testEntity:
		if v, ok := h.val.(testEntity); ok {
			*d = v
		}
	case *sliceWrapper[testEntity]:
		if v, ok := h.val.(sliceWrapper[testEntity]); ok {
			*d = v
		}
	case *foundWrapper[testEntity]:
		if v, ok := h.val.(foundWrapper[testEntity]); ok {
			*d = v
		}
	}
	return nil
}

func (h *hitCache) Set(context.Context, string, interface{}, time.Duration) error { return nil }
func (h *hitCache) Delete(context.Context, string) error                          { return nil }
