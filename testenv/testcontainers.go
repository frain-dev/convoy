package testenv

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/testcontainers/testcontainers-go/log"
)

type testcontainersLogger struct {
	logger *slog.Logger
}

func (t *testcontainersLogger) Printf(format string, v ...any) {
	t.logger.Log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, v...))
}

func NewTestcontainersLogger() log.Logger {
	return &testcontainersLogger{
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}
