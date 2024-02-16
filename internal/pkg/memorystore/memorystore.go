package memorystore

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/frain-dev/convoy/pkg/log"
)

var DefaultStore = Store{
	tables: make(map[string]*Table),
}

type Store struct {
	Syncer
	mutex  sync.RWMutex
	tables map[string]*Table
}

func (s *Store) Register(name string, table *Table) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var tables map[string]*Table

	if s.tables != nil {
		tables = s.tables
	}

	_, ok := tables[name]
	if ok {
		return errors.New("table already registered")
	}

	tables[name] = table

	return nil
}

// TODO(subomi): Improve to have table specific sync intervals.
func (s *Store) Sync(ctx context.Context, interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)

	for range ticker.C {
		log.Info("syncing...")
		// iterate through tables and sync.
		for _, table := range s.tables {
			_ = table.Sync(ctx)
		}
	}
}
