package testenv

import (
	"os"
	"testing"

	"github.com/frain-dev/convoy/pkg/log"
)

// TestingT is an interface that matches the testing.T and testing.B types
type TestingT interface {
	Helper()
	Fatalf(format string, args ...interface{})
	Logf(format string, args ...interface{})
}

func NewLogger(t *testing.T) *log.Logger {
	t.Helper()
	return log.NewLogger(os.Stderr)
}
