package pubsub

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
	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Starting Google Pub/Sub test infrastructure setup...\n")

	// Google Pub/Sub tests need Postgres + Redis + Pub/Sub Emulator
	res, cleanup, err := testenv.Launch(
		context.Background(),
		testenv.WithPubSub(),
	)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "TestMain: Failed to launch test infrastructure: %v\n", err)
		log.Fatalf("Failed to launch test infrastructure: %v", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Google Pub/Sub infrastructure launched successfully\n")
	infra = res

	// Set PUBSUB_EMULATOR_HOST for all tests that need Google Pub/Sub emulator
	// This must be set BEFORE any test runs so the pubsub package initialization sees it
	if res.NewPubSubEmulatorHost != nil {
		emulatorHost := (*res.NewPubSubEmulatorHost)(nil) // Pass nil since factory handles it
		os.Setenv("PUBSUB_EMULATOR_HOST", emulatorHost)
		_, _ = fmt.Fprintf(os.Stderr, "TestMain: Set PUBSUB_EMULATOR_HOST=%s\n", emulatorHost)
		// Verify it was set
		verifyHost := os.Getenv("PUBSUB_EMULATOR_HOST")
		_, _ = fmt.Fprintf(os.Stderr, "TestMain: Verified PUBSUB_EMULATOR_HOST=%s\n", verifyHost)
	}

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Running tests...\n")
	code := m.Run()

	_, _ = fmt.Fprintf(os.Stderr, "TestMain: Cleaning up...\n")
	if err := cleanup(); err != nil {
		log.Fatalf("Failed to cleanup test infrastructure: %v", err)
	}

	os.Exit(code)
}
