package memorystore

import (
	"context"
	"strings"
	"sync"
)

type Key string

func NewKey(prefix, hash string) Key {
	var key strings.Builder

	key.WriteString(prefix)
	key.WriteString(delim)
	key.WriteString(hash)

	return Key(key.String())
}

func (k Key) String() string {
	return string(k)
}

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

	sync.RWMutex
	rows   map[Key]*Row
	syncer Syncer
}

func NewTable(opts ...Option) *Table {
	t := &Table{rows: make(map[Key]*Row)}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

func (t *Table) Get(key Key) *Row {
	t.RLock()
	defer t.RUnlock()

	return t.getInternal(key)
}

// Checks if an item exists in the table.
func (t *Table) Exists(key Key) bool {
	t.RLock()
	defer t.RUnlock()

	return t.existInternal(key)
}

// This assumes the caller has called Lock()
func (t *Table) getInternal(key Key) *Row {
	value, ok := t.rows[key]
	if !ok {
		return nil
	}

	return value
}

// This assumes the caller has called Lock()
func (t *Table) existInternal(key Key) bool {
	_, ok := t.rows[key]
	return ok
}

// Add a new item if it doesn't exist.
func (t *Table) Add(key Key, value interface{}) *Row {
	t.Lock()
	defer t.Unlock()

	row := t.getInternal(key)
	if row != nil {
		return row
	}

	row = &Row{key: string(key), value: value}
	t.rows[key] = row
	return row
}

// Removes an item from the store.
func (t *Table) Delete(key Key) {
	t.Lock()
	defer t.Unlock()

	delete(t.rows, key)
}

func (t *Table) GetKeys() []Key {
	t.RLock()
	defer t.RUnlock()

	var keys []Key
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

func Difference(a, b []Key) []Key {
	mb := make(map[Key]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}

	var diff []Key
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}

	return diff
}
