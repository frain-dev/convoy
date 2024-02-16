package memorystore

import (
	"context"
	"errors"
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
	t := &Table{}

	for _, opt := range opts {
		opt(t)
	}

	return t, nil
}

func (t *Table) GetAll() []*Row {
	var rows []*Row
	for _, v := range t.rows {
		rows = append(rows, v)
	}

	return rows
}

func (t *Table) Get(key string) interface{} {
	value, ok := t.rows[key]
	if !ok {
		return nil
	}

	return value
}

// Checks if an item exists in the table.
func (t *Table) Exists(key string) bool {
	_, ok := t.rows[key]
	return ok
}

// Add new items and emit changes
func (t *Table) Add(key string, value interface{}) error {
	t.Lock()
	defer t.Unlock()

	if t.Exists(key) {
		return errors.New("key exists in table already")
	}

	t.rows[key] = &Row{key: key, value: value}
	return nil
}

// Removes an item from the store.
func (t *Table) Delete(key interface{}) error {
	t.Lock()
	defer t.Unlock()

	return nil
}

func (t *Table) GetKeys() []string {
	var keys []string
	for k, _ := range t.rows {
		keys = append(keys, k)
	}

	return keys
}

func (t *Table) Sync(ctx context.Context) error {
	return t.syncer.SyncChanges(ctx, t)
}
