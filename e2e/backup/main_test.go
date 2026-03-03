package backup

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
)

var (
	infra       *testenv.Environment
	testMutex   sync.Mutex
	portCounter atomic.Uint32
)

func TestMain(m *testing.M) {
	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Starting Backup test infrastructure setup...\n")

	// Backup tests need Postgres + Redis + MinIO
	res, cleanup, err := testenv.Launch(
		context.Background(),
		testenv.WithMinIO(),
	)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "TestMain: Failed to launch test infrastructure: %v\n", err)
		log.Fatalf("Failed to launch test infrastructure: %v", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Backup infrastructure launched successfully\n")
	infra = res

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Running tests...\n")
	code := m.Run()

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Cleaning up...\n")
	if err := cleanup(); err != nil {
		log.Fatalf("Failed to cleanup test infrastructure: %v", err)
	}

	os.Exit(code)
}
