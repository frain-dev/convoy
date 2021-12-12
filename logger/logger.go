package logger

import (
	"errors"

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

func NewLogger(cfg config.LoggerConfiguration) (Logger, error) {

	if cfg.Type != config.ConsoleLoggerProvider {
		return nil, errors.New("Logger is not supported")
	}

	switch cfg.Type {
	case config.ConsoleLoggerProvider:
		lo := NewConsoleLogger(cfg)
		return lo, nil
	}

	return nil, nil
}
