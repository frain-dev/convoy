package worker

import (
	"context"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type ConsumerConfig struct {
	ConsumerPoolSize int
	Queue            queue.Queuer
	Logger           log.StdLogger
	LogLevel         log.Level
}

type Consumer struct {
	queue queue.Queuer
	mux   *asynq.ServeMux
	srv   *asynq.Server
	log   log.StdLogger
}

func NewConsumer(ctx context.Context, cfg ConsumerConfig) *Consumer {
	cfg.Logger.Infof("The consumer pool size has been set to %d.", cfg.ConsumerPoolSize)

	var opts asynq.RedisConnOpt

	if len(cfg.Queue.Options().RedisAddress) == 1 {
		opts = cfg.Queue.Options().RedisClient
	} else {
		opts = asynq.RedisClusterClientOpt{
			Addrs: cfg.Queue.Options().RedisAddress,
		}
	}

	srv := asynq.NewServer(
		opts,
		asynq.Config{
			Concurrency: cfg.ConsumerPoolSize,
			BaseContext: func() context.Context {
				return ctx
			},
			Queues: cfg.Queue.Options().Names,
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
			Logger:         cfg.Logger,
			LogLevel:       getLogLevel(cfg.LogLevel),
		},
	)

	mux := asynq.NewServeMux()

	return &Consumer{
		queue: cfg.Queue,
		log:   cfg.Logger,
		mux:   mux,
		srv:   srv,
	}
}

func (c *Consumer) Start() {
	if err := c.srv.Start(c.mux); err != nil {
		c.log.WithError(err).Fatal("error starting worker")
	}
}

func (c *Consumer) RegisterHandlers(taskName convoy.TaskName, handlerFn func(context.Context, *asynq.Task) error, tel *telemetry.Telemetry) {
	c.mux.HandleFunc(string(taskName), c.loggingMiddleware(asynq.HandlerFunc(handlerFn), tel).ProcessTask)
}

func (c *Consumer) Stop() {
	c.srv.Stop()
	c.srv.Shutdown()
}

func (c *Consumer) loggingMiddleware(h asynq.Handler, tel *telemetry.Telemetry) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
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
