package worker

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/pkg/log"
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
	log        log.StdLogger
	jobTracker JobTracker // optional, used only in E2E tests
}

func NewConsumer(ctx context.Context, consumerPoolSize int, q queue.Queuer, lo log.StdLogger, level log.Level) *Consumer {
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
		// Record job ID if tracker is set (for E2E tests)
		if c.jobTracker != nil {
			c.jobTracker.RecordJob(t)
		}

		traceProvider := otel.GetTracerProvider()
		tracer := traceProvider.Tracer("asynq.workers")

		newCtx, span := tracer.Start(ctx, t.Type())
		span.SetStatus(codes.Ok, "OK")
		defer span.End()

		err := h.ProcessTask(newCtx, t)
		if err != nil {
			c.log.WithError(err).WithField("job", t.Type()).Error("job failed")
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

func getLogLevel(lvl log.Level) asynq.LogLevel {
	switch lvl {
	case log.DebugLevel:
		return asynq.DebugLevel
	case log.InfoLevel:
		return asynq.InfoLevel
	case log.WarnLevel:
		return asynq.WarnLevel
	case log.ErrorLevel:
		return asynq.ErrorLevel
	case log.FatalLevel:
		return asynq.FatalLevel
	default:
		return asynq.InfoLevel
	}
}
