package log

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

func Debug(args ...interface{}) {
	stdLogger.entry.Debug(args...)
}

func Info(args ...interface{}) {
	stdLogger.entry.Info(args...)
}

func Warn(args ...interface{}) {
	stdLogger.entry.Warn(args...)
}

func Error(args ...interface{}) {
	stdLogger.entry.Error(args...)
}

func Fatal(args ...interface{}) {
	stdLogger.entry.Fatal(args...)
}

func Debugf(format string, args ...interface{}) {
	stdLogger.Debug(fmt.Sprintf(format, args...))
}

func Infof(format string, args ...interface{}) {
	stdLogger.Info(fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...interface{}) {
	stdLogger.Warn(fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...interface{}) {
	stdLogger.Error(fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...interface{}) {
	stdLogger.Fatal(fmt.Sprintf(format, args...))
}

func WithError(err error) *logrus.Entry {
	return stdLogger.entry.WithError(err)
}

func WithLogger() *logrus.Logger {
	return stdLogger.logger
}

// SetLevel sets the logger levestdLogger.
// It panics if v is less than DebugLevel or greater than FatalLevestdLogger.
func SetLevel(v Level) {
	lvl, err := v.ToLogrusLevel()
	if err != nil {
		panic(err)
	}

	stdLogger.logger.SetLevel(lvl)
}

// WithField sets logger fields
func SetPrefix(value interface{}) {
	stdLogger.entry = stdLogger.entry.WithField("source", value)
}
