package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/olamilekan000/surge/surge"
	"github.com/olamilekan000/surge/surge/job"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/redis"
)

type JobTracker interface {
	RecordJob(job *job.JobEnvelope)
}

type Consumer struct {
	queue      queue.Queuer
	client     *surge.Client
	log        log.StdLogger
	ctx        context.Context
	cancel     context.CancelFunc
	jobTracker JobTracker // optional, used only in E2E tests
	handlers   map[string]func(context.Context, *job.JobEnvelope) error
	telemetry  map[string]*telemetry.Telemetry
}

func NewConsumer(ctx context.Context, consumerPoolSize int, q queue.Queuer, lo log.StdLogger, level log.Level) *Consumer {
	lo.Infof("The consumer pool size has been set to %d.", consumerPoolSize)

	redisQueue, ok := q.(*redis.RedisQueue)
	if !ok {
		lo.Fatal("queue must be a RedisQueue for surge")
	}

	consumerCtx, cancel := context.WithCancel(ctx)

	client := redisQueue.Client()

	c := &Consumer{
		queue:     q,
		client:    client,
		log:       lo,
		ctx:       consumerCtx,
		cancel:    cancel,
		handlers:  make(map[string]func(context.Context, *job.JobEnvelope) error),
		telemetry: make(map[string]*telemetry.Telemetry),
	}

	return c
}

func (c *Consumer) Start() {
	go func() {
		if err := c.client.Consume(c.ctx); err != nil {
			if err != context.Canceled {
				c.log.WithError(err).Error("error consuming jobs")
			}
		}
	}()
}

func (c *Consumer) RegisterHandlers(taskName convoy.TaskName, handlerFn func(context.Context, *job.JobEnvelope) error, tel *telemetry.Telemetry) {
	taskNameStr := string(taskName)
	c.handlers[taskNameStr] = handlerFn
	if tel != nil {
		c.telemetry[taskNameStr] = tel
	}
	c.log.Infof("Registered handler for task: %s", taskNameStr)
}

func (c *Consumer) RegisterDispatcherHandler() {
	c.log.Infof("Registering %d handlers with surge client", len(c.handlers))

	for taskName, handlerFn := range c.handlers {
		c.log.Infof("Registering handler with surge for task: %s", taskName)

		wrappedHandler := func(ctx context.Context, jobEnvelope *job.JobEnvelope) error {
			var taskPayload redis.TaskPayload
			err := msgpack.DecodeMsgPack(jobEnvelope.Args, &taskPayload)
			if err != nil {
				err = json.Unmarshal(jobEnvelope.Args, &taskPayload)
				if err != nil {
					c.log.WithError(err).Error("failed to unmarshal task payload")
					return err
				}
			}

			unwrappedJob := &job.JobEnvelope{
				ID:        jobEnvelope.ID,
				Topic:     jobEnvelope.Topic,
				Args:      taskPayload.Payload,
				Namespace: jobEnvelope.Namespace,
				Queue:     jobEnvelope.Queue,
				State:     jobEnvelope.State,
				CreatedAt: jobEnvelope.CreatedAt,
			}

			return handlerFn(ctx, unwrappedJob)
		}

		c.client.Handle(taskName, wrappedHandler)
	}

	registeredHandlers, err := c.client.GetRegisteredHandlers(context.Background())
	if err != nil {
		c.log.WithError(err).Error("failed to get registered handlers")
	}
	c.log.Infof("Successfully registered %d handlers with surge: %v", len(registeredHandlers), registeredHandlers)
}

func (c *Consumer) Stop() {
	c.cancel()
	if err := c.client.Shutdown(context.Background()); err != nil {
		c.log.WithError(err).Error("error shutting down surge client")
	}
	if err := c.client.Close(); err != nil {
		c.log.WithError(err).Error("error closing surge client")
	}
}

func (c *Consumer) SetJobTracker(tracker JobTracker) {
	c.jobTracker = tracker
}

func (c *Consumer) dispatcherHandler() surge.HandlerFunc {
	return func(ctx context.Context, jobEnvelope *job.JobEnvelope) error {
		if c.jobTracker != nil {
			c.jobTracker.RecordJob(jobEnvelope)
		}

		var taskPayload redis.TaskPayload
		if err := json.Unmarshal(jobEnvelope.Args, &taskPayload); err != nil {
			c.log.WithError(err).Error("failed to unmarshal task payload")
			return err
		}

		handlerFn, exists := c.handlers[taskPayload.TaskName]
		if !exists {
			c.log.WithError(fmt.Errorf("no handler registered for task %s", taskPayload.TaskName)).
				WithField("available_handlers", c.getRegisteredHandlerNames()).
				Error("no handler registered for task")
			return nil
		}

		traceProvider := otel.GetTracerProvider()
		tracer := traceProvider.Tracer("surge.workers")

		newCtx, span := tracer.Start(ctx, taskPayload.TaskName)
		span.SetStatus(codes.Ok, "OK")
		defer span.End()

		wrappedJob := &job.JobEnvelope{
			ID:        jobEnvelope.ID,
			Topic:     jobEnvelope.Topic,
			Args:      taskPayload.Payload,
			Namespace: jobEnvelope.Namespace,
			Queue:     jobEnvelope.Queue,
			State:     jobEnvelope.State,
			CreatedAt: jobEnvelope.CreatedAt,
		}

		err := handlerFn(newCtx, wrappedJob)
		if err != nil {
			c.log.WithError(err).WithField("job", taskPayload.TaskName).Error("job failed")
			return err
		}

		if tel, ok := c.telemetry[taskPayload.TaskName]; ok {
			switch convoy.TaskName(taskPayload.TaskName) {
			case convoy.EventProcessor:
			case convoy.CreateEventProcessor:
			case convoy.CreateDynamicEventProcessor:
				_ = tel.Capture(newCtx)
			}
		}

		return nil
	}
}

func (c *Consumer) getRegisteredHandlerNames() []string {
	names := make([]string, 0, len(c.handlers))
	for name := range c.handlers {
		names = append(names, name)
	}
	return names
}
