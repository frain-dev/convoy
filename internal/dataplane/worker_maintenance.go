package dataplane

import (
	"context"
	"sync"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/backup_collector"
	"github.com/frain-dev/convoy/worker"
)

// maintenanceConsumer acknowledges suspension only after queue handlers and
// the backup collector have stopped. Unflushed backup data remains in WAL;
// restarting the collector resumes from its durable replication position.
type maintenanceConsumer struct {
	mu        sync.Mutex
	ctx       context.Context
	consumer  *worker.Consumer
	collector *backup_collector.BackupCollector
	running   bool
}

func (m *maintenanceConsumer) Capabilities() ([]string, []string) { return m.consumer.Capabilities() }
func (m *maintenanceConsumer) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	if err := m.consumer.Start(); err != nil {
		return err
	}
	if m.collector != nil {
		if err := m.collector.Start(m.ctx); err != nil {
			_ = m.consumer.Suspend()
			return err
		}
	}
	m.running = true
	return nil
}
func (m *maintenanceConsumer) Suspend() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.consumer.Suspend(); err != nil {
		return err
	}
	if m.running && m.collector != nil {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(m.ctx), 35*time.Second)
		m.collector.Stop(ctx)
		cancel()
	}
	m.running = false
	return nil
}
