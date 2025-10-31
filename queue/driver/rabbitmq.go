package driver

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
)

type RabbitQueue struct {
	opts        queue.QueueOptions
	ch          *amqp.Channel
	ex          string
	mu          sync.RWMutex
	channelFunc func() *amqp.Channel
}

func NewQueue(opts queue.QueueOptions, ch *amqp.Channel, exchange string) *RabbitQueue {
	rq := &RabbitQueue{opts: opts, ch: ch, ex: exchange}
	if ch != nil {
		rq.channelFunc = func() *amqp.Channel { return ch }
	}
	return rq
}

func (q *RabbitQueue) UpdateChannel(ch *amqp.Channel) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ch = ch
	if ch != nil {
		q.channelFunc = func() *amqp.Channel { return ch }
	}
}

func (q *RabbitQueue) Write(ctx context.Context, taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	q.mu.RLock()
	ch := q.getChannel()
	q.mu.RUnlock()

	if ch == nil {
		log.FromContext(ctx).WithError(errors.New("rabbitmq channel not initialized")).Error("rabbitmq: failed to write job to queue")
		return errors.New("rabbitmq channel not initialized")
	}

	// Creates a temporary delay queue for the job if it has a delay
	if job.Delay > 0 {
		delayMs := int(job.Delay / 1e6)
		delayQueue := string(queueName) + ".delay." + strconv.Itoa(delayMs)
		args := amqp.Table{
			"x-message-ttl":             int32(delayMs),
			"x-dead-letter-exchange":    q.ex,
			"x-dead-letter-routing-key": string(queueName),
		}
		if _, err := ch.QueueDeclare(delayQueue, true, false, false, false, args); err != nil {
			log.FromContext(ctx).WithError(err).Error("rabbitmq: failed to declare delay queue")
			return err
		}
		return ch.PublishWithContext(
			ctx,
			"",         // default exchange
			delayQueue, // route to delay queue
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/octet-stream",
				Headers:      amqp.Table{"task_type": string(taskName)},
				Body:         job.Payload,
				DeliveryMode: amqp.Persistent,
			},
		)
	}

	headers := amqp.Table{
		"task_type": string(taskName),
	}

	return ch.PublishWithContext(
		ctx,
		q.ex,              // exchange
		string(queueName), // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType:  "application/octet-stream",
			Headers:      headers,
			Body:         job.Payload,
			DeliveryMode: amqp.Persistent,
		},
	)
}

func (q *RabbitQueue) getChannel() *amqp.Channel {
	if q.ch == nil || q.ch.IsClosed() {
		if q.channelFunc != nil {
			if newCh := q.channelFunc(); newCh != nil && !newCh.IsClosed() {
				q.ch = newCh
				return newCh
			}
		}
		return nil
	}
	return q.ch
}

func (q *RabbitQueue) WriteWithoutTimeout(ctx context.Context, taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	return q.Write(ctx, taskName, queueName, job)
}

func (q *RabbitQueue) Options() queue.QueueOptions { return q.opts }

func (q *RabbitQueue) GetName() string { return "rabbitmq" }

type RabbitWorker struct {
	conn              *amqp.Connection
	ch                *amqp.Channel
	ex                string
	opts              queue.QueueOptions
	log               log.StdLogger
	ctx               context.Context
	handlers          map[string]func(context.Context, *asynq.Task) error
	prefetch          int
	done              chan struct{}
	mu                sync.RWMutex
	reconnectDelay    time.Duration
	maxReconnectDelay time.Duration
	queue             *RabbitQueue
	consumers         map[string]bool
	consumersMu       sync.Mutex
}

func NewRabbitWorker(ctx context.Context, ch *amqp.Channel, exchange string, opts queue.QueueOptions, lo log.StdLogger, level log.Level) *RabbitWorker {
	if l, ok := lo.(*log.Logger); ok {
		l.SetLevel(level)
	}
	prefetch := 100
	if opts.Extra != nil {
		if v, ok := opts.Extra["prefetch"]; ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				prefetch = n
			}
		}
	}
	return &RabbitWorker{
		ch:                ch,
		ex:                exchange,
		opts:              opts,
		log:               lo,
		ctx:               ctx,
		handlers:          map[string]func(context.Context, *asynq.Task) error{},
		prefetch:          prefetch,
		done:              make(chan struct{}),
		reconnectDelay:    5 * time.Second,
		maxReconnectDelay: 60 * time.Second,
		consumers:         make(map[string]bool),
	}
}

func (w *RabbitWorker) SetQueue(queue *RabbitQueue) {
	w.queue = queue
}

func (w *RabbitWorker) RegisterHandlers(taskName convoy.TaskName, handler func(context.Context, *asynq.Task) error, _ *telemetry.Telemetry) {
	w.handlers[string(taskName)] = handler
}

func (w *RabbitWorker) getChannel() *amqp.Channel {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.ch
}

func (w *RabbitWorker) DeclareQueues() error {
	ch := w.getChannel()
	if ch == nil {
		w.log.Warn("rabbitmq worker channel is nil; cannot declare queues")
		return errors.New("rabbitmq channel not initialized")
	}

	if err := ch.ExchangeDeclare(w.ex, "direct", true, false, false, false, nil); err != nil {
		w.log.WithError(err).Errorf("rabbitmq: failed to declare main exchange %s", w.ex)
		return err
	}

	dlxName := w.ex + ".dlx"
	if err := ch.ExchangeDeclare(dlxName, "direct", true, false, false, false, nil); err != nil {
		w.log.WithError(err).Errorf("rabbitmq: failed to declare DLX %s", dlxName)
		return err
	}

	w.log.Infof("rabbitmq: declaring %d queues", len(w.opts.Names))

	for qname := range w.opts.Names {
		dlqName := qname + ".dead"
		if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
			w.log.WithError(err).Errorf("rabbitmq: failed to declare DLQ %s", dlqName)
			continue
		}
		if err := ch.QueueBind(dlqName, qname, dlxName, false, nil); err != nil {
			w.log.WithError(err).Errorf("rabbitmq: failed to bind DLQ %s to DLX %s with routing key %s", dlqName, dlxName, qname)
			continue
		}

		args := amqp.Table{
			"x-dead-letter-exchange":    dlxName,
			"x-dead-letter-routing-key": qname,
		}
		_, err := ch.QueueDeclare(qname, true, false, false, false, args)
		if err != nil {
			w.log.WithError(err).Errorf("rabbitmq: failed to declare queue %s", qname)
			continue
		}

		if err := ch.QueueBind(qname, qname, w.ex, false, nil); err != nil {
			w.log.WithError(err).Errorf("rabbitmq: failed to bind queue %s to exchange %s", qname, w.ex)
			continue
		}

		w.log.Infof("rabbitmq: declared queue %s", qname)
	}

	w.log.Info("rabbitmq: all queues and exchanges declared")
	return nil
}

func (w *RabbitWorker) reconnect() error {
	w.mu.Lock()
	if w.ch != nil {
		_ = w.ch.Close()
		w.ch = nil
	}
	if w.conn != nil {
		_ = w.conn.Close()
		w.conn = nil
	}

	cfg, _ := config.Get()
	url := cfg.RabbitMQ.BuildURL()
	if url == "" {
		w.mu.Unlock()
		return errors.New("rabbitmq url not configured")
	}

	currentDelay := w.reconnectDelay
	maxAttempts := 5
	w.mu.Unlock()

	// Retry with exponential backoff
	for i := 0; i < maxAttempts; i++ {
		if i > 0 {
			w.log.Infof("rabbitmq: reconnect attempt %d/%d (waiting %v)", i+1, maxAttempts, currentDelay)
			time.Sleep(currentDelay)
			currentDelay = time.Duration(float64(currentDelay) * 1.5)
			if currentDelay > w.maxReconnectDelay {
				currentDelay = w.maxReconnectDelay
			}
		}

		conn, err := amqp.Dial(url)
		if err != nil {
			w.log.WithError(err).Warnf("rabbitmq: reconnect attempt %d failed to connect", i+1)
			continue
		}

		ch, err := conn.Channel()
		if err != nil {
			conn.Close()
			w.log.WithError(err).Warnf("rabbitmq: reconnect attempt %d failed to open channel", i+1)
			continue
		}

		w.mu.Lock()
		w.conn = conn
		w.ch = ch
		w.mu.Unlock()

		// Re-set QoS
		if err := ch.Qos(w.prefetch, 0, false); err != nil {
			w.log.WithError(err).Error("rabbitmq: failed to set QoS after reconnect")
		}

		if err := w.DeclareQueues(); err != nil {
			w.log.WithError(err).Error("rabbitmq: failed to redeclare queues after reconnect")
		}

		if w.queue != nil {
			w.queue.UpdateChannel(ch)
		}

		w.mu.Lock()
		w.reconnectDelay = 5 * time.Second
		w.mu.Unlock()

		w.log.Infof("rabbitmq: successfully reconnected")
		return nil
	}

	return errors.New("rabbitmq: failed to reconnect after multiple attempts")
}

func (w *RabbitWorker) startConsumerForQueue(queueName string) {
	w.consumersMu.Lock()
	if w.consumers[queueName] {
		w.consumersMu.Unlock()
		return
	}
	w.consumers[queueName] = true
	w.consumersMu.Unlock()

	ch := w.getChannel()
	if ch == nil {
		w.log.Warnf("rabbitmq: channel is nil for queue %s, cannot start consumer", queueName)
		w.consumersMu.Lock()
		w.consumers[queueName] = false
		w.consumersMu.Unlock()
		return
	}

	deliveries, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		w.log.WithError(err).Errorf("rabbitmq: failed to consume queue %s", queueName)
		w.consumersMu.Lock()
		w.consumers[queueName] = false
		w.consumersMu.Unlock()
		return
	}

	w.log.Infof("rabbitmq: started consumer for queue %s", queueName)

	go func(q string, dch <-chan amqp.Delivery) {
		defer func() {
			w.consumersMu.Lock()
			w.consumers[q] = false
			w.consumersMu.Unlock()
		}()

		for {
			select {
			case <-w.done:
				w.log.Infof("rabbitmq: consumer for queue %s shutting down", q)
				return
			case d, ok := <-dch:
				if !ok {
					w.log.Warnf("rabbitmq: delivery channel for queue %s closed, attempting to reconnect", q)
					if err := w.reconnect(); err != nil {
						w.log.WithError(err).Errorf("rabbitmq: failed to reconnect for queue %s", q)
						return
					}
					go w.startConsumerForQueue(q)
					return
				}

				ttype, _ := d.Headers["task_type"].(string)
				if ttype == "" {
					w.log.Warnf("rabbitmq: received message without task_type header, dropping message from queue %s", q)
					_ = d.Nack(false, false) // Nack without requeue to DLQ
					continue
				}

				h, ok := w.handlers[ttype]
				if !ok {
					w.log.Warnf("rabbitmq: no handler registered for task type %s, dropping message from queue %s", ttype, q)
					_ = d.Nack(false, false) // Nack without requeue to DLQ
					continue
				}

				t := asynq.NewTask(ttype, d.Body)
				err := h(w.ctx, t)
				if err != nil {
					w.log.WithError(err).Errorf("rabbitmq: handler error for task %s in queue %s", ttype, q)
					_ = d.Nack(false, false) // Nack without requeue to DLQ
				} else {
					_ = d.Ack(false)
				}
			}
		}
	}(queueName, deliveries)
}

func (w *RabbitWorker) Start() {
	ch := w.getChannel()
	if ch == nil {
		w.log.Warn("rabbitmq worker channel is nil; skipping start")
		return
	}

	if err := ch.Qos(w.prefetch, 0, false); err != nil {
		w.log.WithError(err).Errorf("rabbitmq: failed to set QoS prefetch to %d", w.prefetch)
	}

	go w.monitorConnection()

	for qname := range w.opts.Names {
		go w.startConsumerForQueue(qname)
	}

	w.log.Info("rabbitmq: all queues declared and consumers started")
}

// monitorConnection monitors the connection and channel for close events
func (w *RabbitWorker) monitorConnection() {
	w.mu.RLock()
	conn := w.conn
	ch := w.ch
	w.mu.RUnlock()

	if conn == nil || ch == nil {
		return
	}

	go func() {
		notifyConnClose := conn.NotifyClose(make(chan *amqp.Error, 1))
		for {
			select {
			case <-w.done:
				return
			case err, ok := <-notifyConnClose:
				if !ok {
					w.log.Warn("rabbitmq: connection close channel closed")
					return
				}
				if err != nil {
					w.log.WithError(err).Warn("rabbitmq: connection closed, attempting to reconnect")
					if reconnectErr := w.reconnect(); reconnectErr != nil {
						w.log.WithError(reconnectErr).Error("rabbitmq: failed to reconnect after connection close")
						go w.monitorConnection()
						return
					}
					for qname := range w.opts.Names {
						go w.startConsumerForQueue(qname)
					}
					go w.monitorConnection()
					return
				}
			}
		}
	}()

	go func() {
		notifyChClose := ch.NotifyClose(make(chan *amqp.Error, 1))
		for {
			select {
			case <-w.done:
				return
			case err, ok := <-notifyChClose:
				if !ok {
					w.log.Warn("rabbitmq: channel close channel closed")
					return
				}
				if err != nil {
					w.log.WithError(err).Warn("rabbitmq: channel closed, attempting to reconnect")
					if reconnectErr := w.reconnect(); reconnectErr != nil {
						w.log.WithError(reconnectErr).Error("rabbitmq: failed to reconnect after channel close")
						go w.monitorConnection()
						return
					}

					for qname := range w.opts.Names {
						go w.startConsumerForQueue(qname)
					}

					go w.monitorConnection()
					return
				}
			}
		}
	}()
}

func (w *RabbitWorker) Stop() {
	select {
	case <-w.done:
	default:
		close(w.done)
	}
	if w.ch != nil {
		_ = w.ch.Close()
	}
	if w.conn != nil {
		_ = w.conn.Close()
	}
}

type RabbitDriver struct {
	queuer queue.Queuer
	worker *RabbitWorker
}

func NewRabbitDriver(ctx context.Context, opts queue.QueueOptions, lo log.StdLogger, level log.Level) QueueDriver {
	cfg, _ := config.Get()
	url := cfg.RabbitMQ.BuildURL()
	var conn *amqp.Connection
	var ch *amqp.Channel
	var err error
	if url != "" {
		conn, err = amqp.Dial(url)
		if err != nil {
			lo.WithError(err).Warn("rabbitmq: failed to connect; driver will be inert")
		} else {
			ch, err = conn.Channel()
			if err != nil {
				lo.WithError(err).Warn("rabbitmq: failed to open channel; driver will be inert")
			}
		}
	}
	exchange := cfg.RabbitMQ.Exchange
	if exchange == "" {
		exchange = "convoy"
	}
	rq := NewQueue(opts, ch, exchange)
	rw := NewRabbitWorker(ctx, ch, exchange, opts, lo, level)
	rw.conn = conn
	rw.SetQueue(rq)
	return &RabbitDriver{queuer: rq, worker: rw}
}

func (d *RabbitDriver) Queuer() queue.Queuer { return d.queuer }
func (d *RabbitDriver) Worker() QueueWorker  { return d.worker }
func (d *RabbitDriver) Name() string         { return "rabbitmq" }
func (d *RabbitDriver) Initialize() error {
	if d.worker != nil {
		return d.worker.DeclareQueues()
	}
	return errors.New("rabbitmq worker is nil")
}
