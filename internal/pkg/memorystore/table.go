package memorystore

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

const delim = ":"

type ITable interface {
	GetItems() []*Row
}

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

	namespace string
	sync.RWMutex
	rows   map[string]*Row
	syncer Syncer
}

func NewTable(namespace string, opts ...Option) *Table {
	t := &Table{namespace: namespace, rows: make(map[string]*Row)}

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
	value, ok := t.rows[t.generateNamespacedKey(key)]
	if !ok {
		return nil
	}

	return value
}

// This assumes the caller has called Lock()
func (t *Table) existInternal(key string) bool {
	_, ok := t.rows[t.generateNamespacedKey(key)]
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
	t.rows[t.generateNamespacedKey(key)] = row
	return row
}

// Removes an item from the store.
func (t *Table) Delete(key string) {
	t.Lock()
	defer t.Unlock()

	delete(t.rows, t.generateNamespacedKey(key))
}

func (t *Table) GetKeys() []string {
	t.RLock()
	defer t.RUnlock()

	var keys []string
	for k := range t.rows {
		keys = append(keys, t.generateResourceKey(k))
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

func (t *Table) generateNamespacedKey(key string) string {
	return fmt.Sprintf("%s%s%s", t.namespace, delim, key)
}

func (t *Table) generateResourceKey(key string) string {
	return strings.Split(key, delim)[1]
}

func (t *Table) Sync(ctx context.Context) error {
	return t.syncer.SyncChanges(ctx, t)
}
