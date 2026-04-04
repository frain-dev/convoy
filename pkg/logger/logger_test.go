package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/frain-dev/convoy"
)

func newTestLogger(buf *bytes.Buffer, level Level) *SlogLogger {
	jsonHandler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	handler := newTraceHandler(jsonHandler)
	return &SlogLogger{
		logger: slog.New(handler),
	}
}

func parseLogEntry(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v\nraw: %s", err, buf.String())
	}
	return entry
}

func TestInfoContext_WithTraceContext(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newTestLogger(buf, LevelDebug)

	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	l.InfoContext(ctx, "test message", "key", "value")

	entry := parseLogEntry(t, buf)

	if entry["trace_id"] != traceID.String() {
		t.Errorf("trace_id = %v, want %v", entry["trace_id"], traceID.String())
	}
	if entry["span_id"] != spanID.String() {
		t.Errorf("span_id = %v, want %v", entry["span_id"], spanID.String())
	}
	if entry["msg"] != "test message" {
		t.Errorf("msg = %v, want %v", entry["msg"], "test message")
	}
	if entry["key"] != "value" {
		t.Errorf("key = %v, want %v", entry["key"], "value")
	}
}

func TestInfoContext_WithRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newTestLogger(buf, LevelDebug)

	ctx := context.WithValue(context.Background(), convoy.RequestIDKey, "req-abc-123")

	l.InfoContext(ctx, "with request id")

	entry := parseLogEntry(t, buf)

	if entry["request_id"] != "req-abc-123" {
		t.Errorf("request_id = %v, want %v", entry["request_id"], "req-abc-123")
	}
}

func TestInfoContext_WithoutContext(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newTestLogger(buf, LevelDebug)

	l.InfoContext(context.Background(), "no trace context")

	entry := parseLogEntry(t, buf)

	if _, ok := entry["trace_id"]; ok {
		t.Errorf("trace_id should not be present, got %v", entry["trace_id"])
	}
	if _, ok := entry["span_id"]; ok {
		t.Errorf("span_id should not be present, got %v", entry["span_id"])
	}
	if _, ok := entry["request_id"]; ok {
		t.Errorf("request_id should not be present, got %v", entry["request_id"])
	}
}

func TestInfo_BackwardsCompatible(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newTestLogger(buf, LevelDebug)

	l.Info("old style log", "key", "value")

	entry := parseLogEntry(t, buf)

	if entry["msg"] != "old style log" {
		t.Errorf("msg = %v, want %v", entry["msg"], "old style log")
	}
	if entry["key"] != "value" {
		t.Errorf("key = %v, want %v", entry["key"], "value")
	}
	// Source should be a structured object with function, file, line
	source, ok := entry["source"].(map[string]any)
	if !ok {
		t.Fatal("source should be a structured object")
	}
	if source["file"] == nil {
		t.Error("source.file should be present")
	}
	if source["line"] == nil {
		t.Error("source.line should be present")
	}
}

func TestErrorContext_WithFullContext(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newTestLogger(buf, LevelDebug)

	traceID, _ := trace.TraceIDFromHex("abcdef0123456789abcdef0123456789")
	spanID, _ := trace.SpanIDFromHex("abcdef0123456789")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	ctx = context.WithValue(ctx, convoy.RequestIDKey, "req-xyz")

	l.ErrorContext(ctx, "something failed", "error", "timeout")

	entry := parseLogEntry(t, buf)

	if entry["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", entry["level"])
	}
	if entry["trace_id"] != traceID.String() {
		t.Errorf("trace_id = %v, want %v", entry["trace_id"], traceID.String())
	}
	if entry["span_id"] != spanID.String() {
		t.Errorf("span_id = %v, want %v", entry["span_id"], spanID.String())
	}
	if entry["request_id"] != "req-xyz" {
		t.Errorf("request_id = %v, want %v", entry["request_id"], "req-xyz")
	}
}

func TestDebug_SuppressedAtInfoLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newTestLogger(buf, LevelInfo)

	l.Debug("this should be suppressed")

	if buf.Len() != 0 {
		t.Errorf("expected no output for suppressed debug log, got: %s", buf.String())
	}
}

func TestInfo_NoArgs_DoesNotPanic(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newTestLogger(buf, LevelDebug)

	// None of these should panic.
	l.Info()
	l.Debug()
	l.Warn()
	l.Error()

	// No output expected for zero-arg calls.
	if buf.Len() != 0 {
		t.Errorf("expected no output for zero-arg log calls, got: %s", buf.String())
	}
}

func TestSource_PointsToCallerNotLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newTestLogger(buf, LevelDebug)

	l.Info("check source")

	entry := parseLogEntry(t, buf)
	source, ok := entry["source"].(map[string]any)
	if !ok {
		t.Fatal("source should be a structured object")
	}
	file, _ := source["file"].(string)
	if file == "" {
		t.Fatal("source.file should not be empty")
	}
	// Source should point to this test file, not to logger.go
	if file == "logger.go" {
		t.Errorf("source.file should point to the caller, not logger.go; got %s", file)
	}
}
