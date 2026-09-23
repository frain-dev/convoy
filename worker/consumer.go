package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/queue/tracectx"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
)

// JobTracker is an optional interface for capturing job IDs during tests
type JobTracker interface {
	RecordJob(task *asynq.Task)
}

type Consumer struct {
	supportedQueues []string
	supportedTasks  []string
	mux             *asynq.ServeMux
	runner          runner
	log             log.Logger
	jobTracker      JobTracker // optional, used only in E2E tests
	lifecycleMu     sync.Mutex
	newRunner       func() (runner, error)
	ctx             context.Context
	running         bool
	closed          bool
	executionGate   func(context.Context, *asynq.Task, func(context.Context) error) error
}

// ConsumerBackend constructs the provider-specific runner behind Consumer.
// Composition selects one backend and consumers never inspect its provider.
type ConsumerBackend interface {
	newRunner(context.Context, int, map[string]int, *asynq.ServeMux, log.Logger, log.Level) (runner, error)
}

type runner interface {
	Start() error
	Stop()
	Suspend()
}

type redisConsumerBackend struct {
	opts queue.QueueOptions
}

type asynqRunner struct {
	srv      *asynq.Server
	mux      *asynq.ServeMux
	handlers *settlingHandler
}

func (r *asynqRunner) Start() error {
	r.handlers = &settlingHandler{next: r.mux}
	if err := r.srv.Start(r.handlers); err != nil {
		return fmt.Errorf("error starting worker: %w", err)
	}
	return nil
}

func (r *asynqRunner) Stop() {
	r.srv.Stop()
	r.srv.Shutdown()
}

func (r *asynqRunner) Suspend() {
	r.srv.Stop()
	if r.handlers != nil {
		// Asynq's ordinary Shutdown abandons handlers after its timeout.
		// That is appropriate for process exit, but resuming another pool in
		// the same process while an abandoned handler still writes is unsafe.
		r.handlers.closeAndWait()
	}
	r.srv.Shutdown()
}

// The closed gate belongs to one runner generation and is never reopened.
// A task claimed just before Stop but not yet handed to us remains with Asynq
// until shutdown returns it to pending. It must not enter application code.
type settlingHandler struct {
	next   asynq.Handler
	mu     sync.Mutex
	closed bool
	active sync.WaitGroup
}

func (h *settlingHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		<-ctx.Done()
		return ctx.Err()
	}
	h.active.Add(1)
	h.mu.Unlock()
	defer h.active.Done()
	return h.next.ProcessTask(ctx, t)
}

func (h *settlingHandler) closeAndWait() {
	h.mu.Lock()
	h.closed = true
	h.mu.Unlock()
	h.active.Wait()
}

func NewRedisConsumerBackend(opts queue.QueueOptions) ConsumerBackend {
	return &redisConsumerBackend{opts: opts}
}

func NewPostgresConsumerBackend(q PostgresConsumerQueue) ConsumerBackend {
	return &postgresConsumerBackend{queue: q}
}

func NewConsumer(ctx context.Context, consumerPoolSize int, queueNames map[string]int, backend ConsumerBackend, lo log.Logger, level log.Level) (*Consumer, error) {
	if backend == nil {
		return nil, errors.New("consumer backend is required")
	}
	lo.Infof("The consumer pool size has been set to %d.", consumerPoolSize)

	mux := asynq.NewServeMux()
	// Recreating a stopped runner must use the original queue selection. A
	// caller mutating its map later must not retarget a resumed consumer.
	names := maps.Clone(queueNames)
	makeRunner := func() (runner, error) {
		return backend.newRunner(ctx, consumerPoolSize, maps.Clone(names), mux, lo, level)
	}
	r, err := makeRunner()
	if err != nil {
		return nil, err
	}
	c := &Consumer{
		supportedQueues: slices.Sorted(maps.Keys(names)),
		log:             lo,
		mux:             mux,
		runner:          r,
		newRunner:       makeRunner,
		ctx:             ctx,
	}

	return c, nil
}

func (b *redisConsumerBackend) newRunner(ctx context.Context, consumerPoolSize int, queueNames map[string]int, mux *asynq.ServeMux, lo log.Logger, level log.Level) (runner, error) {
	queueOpts := b.opts
	var opts asynq.RedisConnOpt

	if queueOpts.RedisFailoverOpt != nil {
		opts = *queueOpts.RedisFailoverOpt
	} else if len(queueOpts.RedisAddress) == 1 && queueOpts.RedisClient != nil {
		opts = queueOpts.RedisClient
	} else if len(queueOpts.RedisAddress) > 1 {
		opts = asynq.RedisClusterClientOpt{
			Addrs: queueOpts.RedisAddress,
		}
	} else {
		return nil, errors.New("redis consumer connection is required")
	}

	serverConfig := asynq.Config{
		Concurrency: consumerPoolSize,
		BaseContext: func() context.Context {
			return ctx
		},
		Queues:         queueNames,
		IsFailure:      isCountedFailure,
		RetryDelayFunc: task.GetRetryDelay,
		Logger:         lo,
		LogLevel:       getLogLevel(level),
	}
	var srv *asynq.Server
	if queueOpts.RedisFailoverOpt == nil && len(queueOpts.RedisAddress) == 1 && queueOpts.RedisClient != nil {
		// The broker owns this pool. Suspending one consumer must not close
		// the producer, inspector or another consumer sharing the connection.
		srv = asynq.NewServerFromRedisClient(queueOpts.RedisClient.Client(), serverConfig)
	} else {
		srv = asynq.NewServer(opts, serverConfig)
	}
	return &asynqRunner{srv: srv, mux: mux}, nil
}

func (c *Consumer) Start() error {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.closed {
		return errors.New("consumer is closed")
	}
	if err := c.ctx.Err(); err != nil {
		return err
	}
	if c.running {
		return nil
	}
	if c.runner == nil {
		var err error
		c.runner, err = c.newRunner()
		if err != nil {
			return err
		}
	}
	if err := c.runner.Start(); err != nil {
		c.stopRunner()
		return err
	}
	c.running = true
	return nil
}

func (c *Consumer) RegisterHandlers(taskName convoy.TaskName, handlerFn func(context.Context, *asynq.Task) error) {
	c.supportedTasks = append(c.supportedTasks, string(taskName))
	c.mux.HandleFunc(string(taskName), c.loggingMiddleware(asynq.HandlerFunc(handlerFn)).ProcessTask)
}

func (c *Consumer) Stop() {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	c.stopRunner()
}

// Suspend stops new claims and waits for the runner's existing shutdown path
// to settle its handlers. Start resumes with a fresh runner and the same mux;
// it never re-registers handlers or creates an additional consumer pool.
// This is process-local control, not a deployment-wide drain certificate.
func (c *Consumer) Suspend() error {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.closed {
		return errors.New("consumer is closed")
	}
	if c.runner != nil {
		c.runner.Suspend()
		c.runner = nil
	}
	c.running = false
	return nil
}

func (c *Consumer) stopRunner() {
	if c.runner != nil {
		c.runner.Stop()
		c.runner = nil
	}
	c.running = false
}

// SetJobTracker sets an optional job tracker for E2E tests
func (c *Consumer) SetJobTracker(tracker JobTracker) {
	c.jobTracker = tracker
}

// SetExecutionGate must be called before Start. The gate wraps the complete
// handler so descendants retain its admitted store and operation context.
func (c *Consumer) SetExecutionGate(gate func(context.Context, *asynq.Task, func(context.Context) error) error) {
	c.executionGate = gate
}

func (c *Consumer) loggingMiddleware(h asynq.Handler) asynq.Handler {
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
			return c.runWithSpan(ctx, h, asynq.NewTask(t.Type(), env.payload), env.headers)
		}

		return c.runWithSpan(ctx, h, t, headers)
	})
}

// runWithSpan extracts trace context from headers, opens a worker.task.* span,
// and dispatches to the handler. Shared between the headers-native path and
// the transitional legacy-envelope path.
func (c *Consumer) runWithSpan(ctx context.Context, h asynq.Handler, t *asynq.Task, headers map[string]string) error {
	ctx = tracectx.ExtractContext(ctx, headers)

	tr := otel.GetTracerProvider().Tracer(tracer.TracerNameWorker)

	newCtx, span := tr.Start(ctx, tracer.SpanForTaskName(convoy.TaskName(t.Type())))
	span.SetAttributes(attribute.String(string(tracer.AttrTaskName), t.Type()))
	span.SetStatus(codes.Ok, "OK")
	defer span.End()

	var err error
	if c.executionGate != nil {
		err = c.executionGate(newCtx, t, func(accepted context.Context) error { return h.ProcessTask(accepted, t) })
	} else {
		err = h.ProcessTask(newCtx, t)
	}
	if err != nil {
		c.log.Error("job failed", "error", err, "job", t.Type())
		tracer.RecordError(span, err)
		return err
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

func (c *Consumer) Capabilities() ([]string, []string) {
	return slices.Clone(c.supportedQueues), slices.Clone(c.supportedTasks)
}
