package worker

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"unsafe"

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
		// Unwrap the trace-context envelope (if present) and rebuild the
		// context as a child of the producer's span. Legacy/in-flight tasks
		// without an envelope return the original payload + nil headers, so
		// they simply produce a root span here.
		payload, headers := tracectx.Unwrap(t.Payload())
		ctx = tracectx.ExtractContext(ctx, headers)
		// Swap the payload in place so handlers see the unwrapped bytes.
		// Reconstructing the task with asynq.NewTask would drop the
		// ResultWriter that asynq's dispatcher attached, and any caller
		// that reads t.ResultWriter().TaskID() — e.g. JobTracker in e2e
		// tests — would nil-deref. asynq exposes no public setter for
		// the payload field, hence the reflection/unsafe.
		if !bytes.Equal(t.Payload(), payload) {
			swapTaskPayload(t, payload)
		}

		// Record job ID if tracker is set (for E2E tests). After the
		// in-place unwrap so JobTracker sees the original payload bytes
		// the producer enqueued, not the envelope wrapper.
		if c.jobTracker != nil {
			c.jobTracker.RecordJob(t)
		}

		traceProvider := otel.GetTracerProvider()
		tr := traceProvider.Tracer(tracer.TracerNameWorker)

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
			case convoy.EventProcessor:
			case convoy.CreateEventProcessor:
			case convoy.CreateDynamicEventProcessor:
				_ = tel.Capture(newCtx)
			}
		}

		return nil
	})
}

// swapTaskPayload replaces an asynq.Task's payload in place. asynq.Task's
// payload field is unexported and there is no public setter, so we reach in
// via reflection + unsafe.Pointer. This is preferable to constructing a new
// task because the new task would lack the ResultWriter that asynq's
// processor attached during dispatch — RecordJob and any other consumer-side
// reader of t.ResultWriter() would then nil-deref.
//
// asynq pins to v0.25.x in go.mod; if the Task struct ever renames or moves
// the payload field, this falls back loudly via the reflection panic rather
// than silently corrupting state.
func swapTaskPayload(t *asynq.Task, payload []byte) {
	pf := reflect.ValueOf(t).Elem().FieldByName("payload")
	reflect.NewAt(pf.Type(), unsafe.Pointer(pf.UnsafeAddr())).Elem().SetBytes(payload)
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
