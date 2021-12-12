package logger

import (
	"os"

	"github.com/frain-dev/convoy/config"
	"github.com/sirupsen/logrus"
)

type ConsoleLogger struct {
	Logger *logrus.Logger
}

func NewConsoleLogger(cfg config.LoggerConfiguration) *ConsoleLogger {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
	})

	return &ConsoleLogger{Logger: logger}
}

func (n *ConsoleLogger) Log(level logrus.Level, args ...interface{}) {
	n.Logger.Log(level, args)
}

func (n *ConsoleLogger) Info(args ...interface{}) {
	n.Logger.Info(args)
}

func (n *ConsoleLogger) Debug(args ...interface{}) {
	n.Logger.Debug(args)
}

func (n *ConsoleLogger) Warn(args ...interface{}) {
	n.Logger.Warn(args)
}

func (n *ConsoleLogger) Trace(args ...interface{}) {
	n.Logger.Trace(args)
}

func (n *ConsoleLogger) Error(args ...interface{}) {
	n.Logger.Error(args)
}

func (n *ConsoleLogger) WithLogger() *logrus.Logger {
	return n.Logger
}
