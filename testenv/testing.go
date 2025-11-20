package testenv

import (
	"os"
	"testing"

	"github.com/frain-dev/convoy/pkg/log"
)

func NewLogger(t *testing.T) *log.Logger {
	t.Helper()
	return log.NewLogger(os.Stdout)
}
