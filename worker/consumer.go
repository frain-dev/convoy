package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/queue/tracectx"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/internal/telemetry"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
)

// JobTracker is an optional interface for capturing job IDs during tests
type JobTracker interface {
	RecordJob(task *asynq.Task)
}

type Consumer struct {
	queue      queue.Queuer
	mux        *asynq.ServeMux
	srv        *asynq.Server
	log        log.Logger
	jobTracker JobTracker // optional, used only in E2E tests
}

func NewConsumer(ctx context.Context, consumerPoolSize int, q queue.Queuer, lo log.Logger, level log.Level) *Consumer {
	lo.Infof("The consumer pool size has been set to %d.", consumerPoolSize)

	var opts asynq.RedisConnOpt

	if q.Options().RedisFailoverOpt != nil {
		opts = *q.Options().RedisFailoverOpt
	} else if len(q.Options().RedisAddress) == 1 {
		opts = q.Options().RedisClient
	} else {
		opts = asynq.RedisClusterClientOpt{
			Addrs: q.Options().RedisAddress,
		}
	}

	srv := asynq.NewServer(
		opts,
		asynq.Config{
			Concurrency: consumerPoolSize,
			BaseContext: func() context.Context {
				return ctx
			},
			Queues: q.Options().Names,
			IsFailure: func(err error) bool {
				if _, ok := err.(*task.RateLimitError); ok {
					return false
				}

				if _, ok := err.(*task.CircuitBreakerError); ok {
					return false
				}

				return true
			},
			RetryDelayFunc: task.GetRetryDelay,
			Logger:         lo,
			LogLevel:       getLogLevel(level),
		},
	)

	mux := asynq.NewServeMux()

	return &Consumer{
		queue: q,
		log:   lo,
		mux:   mux,
		srv:   srv,
	}
}

func (c *Consumer) Start() error {
	if err := c.srv.Start(c.mux); err != nil {
		return fmt.Errorf("error starting worker: %w", err)
	}
	return nil
}

func (c *Consumer) RegisterHandlers(taskName convoy.TaskName, handlerFn func(context.Context, *asynq.Task) error, tel *telemetry.Telemetry) {
	c.mux.HandleFunc(string(taskName), c.loggingMiddleware(asynq.HandlerFunc(handlerFn), tel).ProcessTask)
}

func (c *Consumer) Stop() {
	c.srv.Stop()
	c.srv.Shutdown()
}

// SetJobTracker sets an optional job tracker for E2E tests
func (c *Consumer) SetJobTracker(tracker JobTracker) {
	c.jobTracker = tracker
}

func (c *Consumer) loggingMiddleware(h asynq.Handler, tel *telemetry.Telemetry) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		if c.jobTracker != nil {
			c.jobTracker.RecordJob(t)
		}

		// Trace context now rides on asynq.Task.Headers (added in asynq
		// v0.26.0). Producer populates the carrier via tracectx.InjectIntoJob;
		// we extract it back into ctx here so the worker span becomes a child
		// of the producer's. Empty headers (untraced enqueue) → ExtractContext
		// is a no-op and the worker span starts a fresh trace.
		headers := t.Headers()

		// Transitional: tasks enqueued before Epic 10 ride on a custom JSON
		// envelope prefixed with envelopeMagic instead of asynq headers.
		// runLegacy detects them, hands the inner payload to the handler, and
		// extracts trace context from the envelope's "tc" field.
		// TODO(tracing): delete legacyEnvelopeMagic, tryUnwrapLegacyEnvelope,
		// and the runLegacy branch on or after 2026-06-01 — by then every
		// envelope-wrapped payload from the prior deploy has drained.
		if env := tryUnwrapLegacyEnvelope(t.Payload()); env != nil {
			return c.runWithSpan(ctx, h, asynq.NewTask(t.Type(), env.payload), env.headers, tel)
		}

		return c.runWithSpan(ctx, h, t, headers, tel)
	})
}

// runWithSpan extracts trace context from headers, opens a worker.task.* span,
// and dispatches to the handler. Shared between the headers-native path and
// the transitional legacy-envelope path.
func (c *Consumer) runWithSpan(ctx context.Context, h asynq.Handler, t *asynq.Task, headers map[string]string, tel *telemetry.Telemetry) error {
	ctx = tracectx.ExtractContext(ctx, headers)

	tr := otel.GetTracerProvider().Tracer(tracer.TracerNameWorker)

	newCtx, span := tr.Start(ctx, tracer.SpanForTaskName(convoy.TaskName(t.Type())))
	span.SetAttributes(attribute.String(string(tracer.AttrTaskName), t.Type()))
	span.SetStatus(codes.Ok, "OK")
	defer span.End()

	err := h.ProcessTask(newCtx, t)
	if err != nil {
		c.log.Error("job failed", "error", err, "job", t.Type())
		tracer.RecordError(span, err)
		return err
	}

	if tel != nil {
		switch convoy.TaskName(t.Type()) {
		case convoy.EventProcessor, convoy.CreateEventProcessor, convoy.CreateDynamicEventProcessor:
			_ = tel.Capture(newCtx)
		}
	}

	return nil
}

// legacyEnvelopeMagic is the first byte of a payload wrapped by the pre-Epic 10
// tracectx.Wrap. Tasks enqueued before this deploy carry it; tasks enqueued
// after never do.
//
// TODO(tracing): remove this constant, the legacyEnvelope struct, and
// tryUnwrapLegacyEnvelope on or after 2026-06-01 — by then every queue
// has drained the last envelope-wrapped payload from the prior release.
const legacyEnvelopeMagic byte = 0x01

type legacyEnvelope struct {
	headers map[string]string
	payload []byte
}

// tryUnwrapLegacyEnvelope returns a non-nil envelope when body is a payload
// wrapped by the pre-Epic-10 producer, or nil otherwise. Native-headers
// payloads, raw payloads, and any byte sequence that doesn't start with the
// legacy magic byte fall through unchanged.
func tryUnwrapLegacyEnvelope(body []byte) *legacyEnvelope {
	if len(body) == 0 || body[0] != legacyEnvelopeMagic {
		return nil
	}
	var raw struct {
		TC map[string]string `json:"tc"`
		P  []byte            `json:"p"`
	}
	if err := json.Unmarshal(body[1:], &raw); err != nil {
		return nil
	}
	return &legacyEnvelope{headers: raw.TC, payload: raw.P}
}

func getLogLevel(lvl log.Level) asynq.LogLevel {
	switch lvl {
	case log.LevelDebug:
		return asynq.DebugLevel
	case log.LevelInfo:
		return asynq.InfoLevel
	case log.LevelWarn:
		return asynq.WarnLevel
	case log.LevelError:
		return asynq.ErrorLevel
	default:
		return asynq.InfoLevel
	}
}
