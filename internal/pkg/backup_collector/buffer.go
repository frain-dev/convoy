package backup_collector

import (
	"sync"

	"github.com/jackc/pglogrepl"
)

// BufferEntry holds a single WAL INSERT record.
type BufferEntry struct {
	Values map[string]string
}

// Buffer is a thread-safe accumulator for WAL insert records, keyed by table name.
// The streamLoop goroutine appends records; the flushLoop goroutine
// periodically swaps the buffer and processes the old contents.
type Buffer struct {
	mu      sync.Mutex
	records map[string][]BufferEntry // key: table name (e.g. "events")
	maxLSN  pglogrepl.LSN
}

// NewBuffer creates an empty buffer.
func NewBuffer() *Buffer {
	return &Buffer{
		records: make(map[string][]BufferEntry),
	}
}

// Append adds a record to the buffer. Thread-safe.
func (b *Buffer) Append(tableName string, values map[string]string, lsn pglogrepl.LSN) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.records[tableName] = append(b.records[tableName], BufferEntry{Values: values})
	if lsn > b.maxLSN {
		b.maxLSN = lsn
	}
}

// Swap atomically replaces the buffer with an empty one and returns the
// old contents along with the highest LSN seen.
func (b *Buffer) Swap() (map[string][]BufferEntry, pglogrepl.LSN) {
	b.mu.Lock()
	defer b.mu.Unlock()

	records := b.records
	maxLSN := b.maxLSN

	b.records = make(map[string][]BufferEntry)
	b.maxLSN = 0

	return records, maxLSN
}

// Len returns the total number of buffered records. Thread-safe.
func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := 0
	for _, entries := range b.records {
		n += len(entries)
	}
	return n
}
