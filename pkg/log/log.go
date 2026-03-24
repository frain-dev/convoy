// Package log provides a compatibility shim for convoy-go/v2 SDK which imports
// this package. New code should use pkg/logger instead.
package log

import (
	"io"
	"log/slog"
)

// Level represents a log level for compatibility with convoy-go/v2.
type Level int32

const (
	FatalLevel Level = iota
	ErrorLevel
	WarnLevel
	InfoLevel
	DebugLevel
)

// Logger is a compatibility wrapper around slog.Logger for convoy-go/v2.
type Logger struct {
	handler *slog.Logger
}

// NewLogger creates a new Logger writing to out. For compatibility with convoy-go/v2.
func NewLogger(out io.Writer) *Logger {
	h := slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return &Logger{handler: h}
}

// SetLevel sets the log level. For compatibility with convoy-go/v2.
func (l *Logger) SetLevel(lvl Level) {
	// Level is set at construction in slog; this is a no-op for compatibility.
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	l.handler.Debug(format, v...)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.handler.Error(format, v...)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.handler.Info(format, v...)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	l.handler.Warn(format, v...)
}
