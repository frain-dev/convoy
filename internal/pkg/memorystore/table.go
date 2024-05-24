package memorystore

import (
	"context"
	"sync"
)

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

func NewTable(opts ...Option) *Table {
	t := &Table{rows: make(map[string]*Row)}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

func (t *Table) Get(key string) *Row {
	t.RLock()
	defer t.RUnlock()

	return t.getInternal(key)
}

// Checks if an item exists in the table.
func (t *Table) Exists(key string) bool {
	t.RLock()
	defer t.RUnlock()

	return t.existInternal(key)
}

// This assumes the caller has called Lock()
func (t *Table) getInternal(key string) *Row {
	value, ok := t.rows[key]
	if !ok {
		return nil
	}

	return value
}

// This assumes the caller has called Lock()
func (t *Table) existInternal(key string) bool {
	_, ok := t.rows[key]
	return ok
}

// Add a new item if it doesn't exist.
func (t *Table) Add(key string, value interface{}) *Row {
	t.Lock()
	defer t.Unlock()

	row := t.getInternal(key)
	if row != nil {
		return row
	}

	row = &Row{key: key, value: value}
	t.rows[key] = row
	return row
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

func (t *Table) GetItems() []*Row {
	t.Lock()
	defer t.Unlock()

	var rows []*Row
	for _, row := range t.rows {
		rows = append(rows, row)
	}

	return rows
}

func (t *Table) Sync(ctx context.Context) error {
	return t.syncer.SyncChanges(ctx, t)
}
