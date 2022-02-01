package logger

import (
	"github.com/frain-dev/convoy/config"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

var InfoLevel = "info"

func DefaultLogLevel(lvl string) string {
	if lvl == "" {
		return InfoLevel
	}

	return lvl
}

type Logger interface {
	Log(level logrus.Level, args ...interface{})
	Info(args ...interface{})
	Debug(args ...interface{})
	Warn(args ...interface{})
	Trace(args ...interface{})
	Error(args ...interface{})
	WithLogger() *logrus.Logger
}

type NoopLogger struct {
	Logger *logrus.Logger
}

func (NoopLogger) Log(level logrus.Level, args ...interface{}) {}
func (NoopLogger) Info(args ...interface{})                    {}
func (NoopLogger) Debug(args ...interface{})                   {}
func (NoopLogger) Warn(args ...interface{})                    {}
func (NoopLogger) Trace(args ...interface{})                   {}
func (NoopLogger) Error(args ...interface{})                   {}
func (n NoopLogger) WithLogger() *logrus.Logger {
	return n.Logger
}

func NewNoopLogger() Logger {
	lo, _ := test.NewNullLogger()
	return &NoopLogger{Logger: lo}
}

func NewLogger(cfg config.LoggerConfiguration) (Logger, error) {
	switch cfg.Type {
	case config.ConsoleLoggerProvider:
		lo, err := NewConsoleLogger(cfg)
		if err != nil {
			return nil, err
		}
		return lo, nil
	default:
		lo, err := NewConsoleLogger(cfg)
		if err != nil {
			return nil, err
		}

		return lo, nil
	}
}

func CanLogHttpRequest(log Logger) bool {
	return log.WithLogger().IsLevelEnabled(logrus.InfoLevel)
}
