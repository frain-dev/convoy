package backup_collector

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/stretchr/testify/require"
)

type rejectedUpload struct{ err error }

func (s rejectedUpload) Upload(context.Context, string, io.Reader) error { return s.err }

func TestFailedUploadReleasesEncoder(t *testing.T) {
	failure := errors.New("storage unavailable")
	collector := NewBackupCollector(nil, "", rejectedUpload{failure}, time.Second, log.New("test", log.LevelInfo), nil)
	finished := make(chan error, 1)
	go func() {
		finished <- collector.flushTable(context.Background(), "events", []BufferEntry{{Values: map[string]string{"id": "synthetic"}}})
	}()
	select {
	case err := <-finished:
		require.ErrorIs(t, err, failure)
	case <-time.After(2 * time.Second):
		t.Fatal("failed upload stranded encoder and would prevent maintenance settlement")
	}
}
