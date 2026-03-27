package sqs

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/frain-dev/convoy/testenv"
)

var (
	infra       *testenv.Environment
	testMutex   sync.Mutex
	portCounter atomic.Uint32
)

func TestMain(m *testing.M) {
	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Starting SQS test infrastructure setup...\n")

	// SQS tests need Postgres + Redis + LocalStack
	res, cleanup, err := testenv.Launch(
		context.Background(),
		testenv.WithLocalStack(),
	)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "TestMain: Failed to launch test infrastructure: %v\n", err)
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: SQS infrastructure launched successfully\n")
	infra = res

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Running tests...\n")
	code := m.Run()

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Cleaning up...\n")
	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup test infrastructure: %v\n", err)
		os.Exit(1)
	}

	os.Exit(code)
}
