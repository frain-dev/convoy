package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

// Level is a type alias for slog.Level so callers don't need to import log/slog.
type Level = slog.Level

// Level constants re-exported from log/slog so callers don't need to import log/slog directly.
const (
	LevelDebug Level = slog.LevelDebug
	LevelInfo  Level = slog.LevelInfo
	LevelWarn  Level = slog.LevelWarn
	LevelError Level = slog.LevelError
)

type Logger interface {
	Info(args ...any)
	Debug(args ...any)
	Warn(args ...any)
	Error(args ...any)
	Fatal(args ...any)
	Infof(format string, args ...any)
	Debugf(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Printf(format string, args ...any)

	InfoContext(ctx context.Context, msg string, args ...any)
	DebugContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
	Log(ctx context.Context, level Level, msg string, args ...any)
}

// SlogLogger is a reusable logger implementation that supports contextual fields
type SlogLogger struct {
	logger *slog.Logger
}

// New creates the single logger instance to be reused
func New(namespace string, level slog.Level) *SlogLogger {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	handler := newTraceHandler(jsonHandler)
	attr := []slog.Attr{slog.String("service", namespace)}
	return &SlogLogger{
		logger: slog.New(handler.WithAttrs(attr)),
	}
}

// log is the core logging method. It checks the level, captures the caller's PC,
// and delegates to the handler. All public methods call this.
// callerSkip=3 accounts for: runtime.Callers, log, and the public method (Info/Error/etc).
func (l *SlogLogger) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if !l.logger.Enabled(ctx, level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = l.logger.Handler().Handle(ctx, r)
}

func (l *SlogLogger) Info(args ...any) {
	if len(args) == 0 {
		return
	}
	l.log(context.Background(), slog.LevelInfo, fmt.Sprint(args[0]), args[1:]...)
}

func (l *SlogLogger) Debug(args ...any) {
	if len(args) == 0 {
		return
	}
	l.log(context.Background(), slog.LevelDebug, fmt.Sprint(args[0]), args[1:]...)
}

func (l *SlogLogger) Warn(args ...any) {
	if len(args) == 0 {
		return
	}
	l.log(context.Background(), slog.LevelWarn, fmt.Sprint(args[0]), args[1:]...)
}

func (l *SlogLogger) Error(args ...any) {
	if len(args) == 0 {
		return
	}
	l.log(context.Background(), slog.LevelError, fmt.Sprint(args[0]), args[1:]...)
}

func (l *SlogLogger) Fatal(args ...any) {
	if len(args) > 0 {
		l.log(context.Background(), slog.LevelError, fmt.Sprint(args[0]), args[1:]...)
	}
	os.Exit(1)
}

func (l *SlogLogger) Infof(format string, args ...any) {
	l.log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
}

func (l *SlogLogger) Debugf(format string, args ...any) {
	l.log(context.Background(), slog.LevelDebug, fmt.Sprintf(format, args...))
}

func (l *SlogLogger) Warnf(format string, args ...any) {
	l.log(context.Background(), slog.LevelWarn, fmt.Sprintf(format, args...))
}

func (l *SlogLogger) Errorf(format string, args ...any) {
	l.log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
}

func (l *SlogLogger) Fatalf(format string, args ...any) {
	l.log(context.Background(), slog.LevelError, fmt.Sprintf(format, args...))
	os.Exit(1)
}

func (l *SlogLogger) Printf(format string, args ...any) {
	l.log(context.Background(), slog.LevelInfo, fmt.Sprintf(format, args...))
}

func (l *SlogLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelInfo, msg, args...)
}

func (l *SlogLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelDebug, msg, args...)
}

func (l *SlogLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelWarn, msg, args...)
}

func (l *SlogLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelError, msg, args...)
}

// Log logs a message at the given level with context. Useful when the level is dynamic.
func (l *SlogLogger) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	l.log(ctx, level, msg, args...)
}

// ParseLevel converts a level string to slog.Level.
// Supported values: "debug", "info", "warn", "warning", "error", "fatal".
func ParseLevel(lvl string) (slog.Level, error) {
	switch strings.ToLower(lvl) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	case "fatal":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("not a valid log level: %q", lvl)
	}
}
