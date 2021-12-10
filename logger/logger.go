package logger

import (
	"github.com/frain-dev/convoy/config"
	"github.com/sirupsen/logrus"
)

type Logger interface {
	Log(level logrus.Level, args ...interface{})
	Info(args ...interface{})
	Debug(args ...interface{})
	Warn(args ...interface{})
	Trace(args ...interface{})
	Error(args ...interface{})
	WithLogger() *logrus.Logger
}

func NewLogger(cfg config.Configuration) (Logger, error) {
	switch cfg.Logger.Type {
	case config.ConsoleLoggerProvider:
		lo := NewConsoleLogger(cfg)
		return lo, nil
	}

	return nil, nil
}
