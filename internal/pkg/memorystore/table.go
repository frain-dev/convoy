package memorystore

import (
	"context"
	"sync"
)

// In ingest.go
// configure table for sources
// sourceTable := memorystore.NewTable(dataLoader)
//
// register table in db.
// err := memorystore.Store.Register(sourceTable)
// handleError(err)
//
// In workers.go
// endpointTable := memorystore.NewTable(endpointLoader)
// store := memorystore.Register(endpointTable)

type Syncer interface {
	SyncChanges(context.Context, *Table) error
}

type Option func(t *Table)

func OptionSyncer(s Syncer) func(*Table) {
	return func(t *Table) {
		t.syncer = s
	}
}

type Table struct {
	Syncer

	sync.RWMutex
	rows   map[string]*Row
	syncer Syncer
}

func NewTable(opts ...Option) (*Table, error) {
	t := &Table{rows: make(map[string]*Row)}

	for _, opt := range opts {
		opt(t)
	}

	return t, nil
}

func (t *Table) GetAll() []*Row {
	t.RLock()
	defer t.RUnlock()

	var rows []*Row
	for _, v := range t.rows {
		rows = append(rows, v)
	}

	return rows
}

func (t *Table) Get(key string) *Row {
	t.RLock()
	defer t.RUnlock()

	value, ok := t.rows[key]
	if !ok {
		return nil
	}

	return value
}

// Checks if an item exists in the table.
func (t *Table) Exists(key string) bool {
	t.RLock()
	defer t.RUnlock()

	return t.existInternal(key)
}

// This assumes the caller has called Lock()
func (t *Table) existInternal(key string) bool {
	_, ok := t.rows[key]
	return ok
}

// Add a new item if it doesn't exist.
func (t *Table) Add(key string, value interface{}) error {
	t.Lock()
	defer t.Unlock()

	if t.existInternal(key) {
		return nil
	}

	t.rows[key] = &Row{key: key, value: value}
	return nil
}

// Removes an item from the store.
func (t *Table) Delete(key string) {
	t.Lock()
	defer t.Unlock()

	delete(t.rows, key)
}

func (t *Table) GetKeys() []string {
	t.RLock()
	defer t.RUnlock()

	var keys []string
	for k := range t.rows {
		keys = append(keys, k)
	}

	return keys
}

func (t *Table) Sync(ctx context.Context) error {
	return t.syncer.SyncChanges(ctx, t)
}
