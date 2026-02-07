package rqm

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	common "github.com/frain-dev/convoy/internal/pkg/pubsub/const"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
)

const (
	DeadLetterExchangeHeader = "x-dead-letter-exchange"

	initialReconnectDelay = 1 * time.Second
	maxReconnectDelay     = 60 * time.Second
	reconnectMultiplier   = 2.0
)

type Amqp struct {
	Cfg         *datastore.AmqpPubSubConfig
	source      *datastore.Source
	workers     int
	ctx         context.Context
	handler     datastore.PubSubHandler
	log         log.StdLogger
	rateLimiter limiter.RateLimiter
	licenser    license.Licenser
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger, rateLimiter limiter.RateLimiter, licenser license.Licenser) *Amqp {
	return &Amqp{
		Cfg:         source.PubSub.Amqp,
		source:      source,
		workers:     source.PubSub.Workers,
		handler:     handler,
		log:         log,
		rateLimiter: rateLimiter,
		licenser:    licenser,
	}
}

func (a *Amqp) Start(ctx context.Context) {
	a.ctx = ctx
	for i := 1; i <= a.workers; i++ {
		go func() {
			defer a.handleError()
			a.consume()
		}()
	}
}

func (a *Amqp) dialer() (*amqp.Connection, error) {
	auth := ""
	if a.Cfg.Auth != nil {
		auth = fmt.Sprintf("%s:%s@", a.Cfg.Auth.User, a.Cfg.Auth.Password)
	}

	connString := fmt.Sprintf("%s://%s%s:%s/%s?heartbeat=30", a.Cfg.Schema, auth, a.Cfg.Host, a.Cfg.Port, *a.Cfg.Vhost)
	conn, err := amqp.Dial(connString)
	if err != nil {
		log.WithError(err).Error("Failed to open connection to amqp")
		return nil, err
	}

	if conn == nil {
		err := fmt.Errorf("failed to instantiate a connection - connection is nil")
		return nil, err
	}

	return conn, nil
}

func (a *Amqp) Verify() error {
	conn, err := a.dialer()
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		log.WithError(err).Error("failed to instantiate a channel")
		return err
	}
	defer ch.Close()

	return nil
}

// consume is the outer reconnection loop that handles connection failures
// and implements exponential backoff between reconnection attempts.
func (a *Amqp) consume() {
	var attempt uint64 = 0

	for {
		select {
		case <-a.ctx.Done():
			a.log.Info("AMQP consumer shutting down due to context cancellation")
			return
		default:
			if attempt > 0 {
				delay := a.calculateBackoff(attempt)
				a.log.Infof("AMQP reconnection attempt %d for queue: %s, waiting %v", attempt, a.Cfg.Queue, delay)
				select {
				case <-a.ctx.Done():
					return
				case <-time.After(delay):
				}
			}

			connected, err := a.consumeWithConnection()
			if err == nil {
				// Graceful shutdown
				return
			}

			// Reset attempt counter if we had a successful connection
			// This ensures fresh backoff after a working connection fails
			if connected {
				attempt = 0
			}

			attempt++
			a.log.WithError(err).Errorf("AMQP connection failed for queue: %s, will reconnect", a.Cfg.Queue)
		}
	}
}

// consumeWithConnection handles a single connection lifecycle.
// Returns (true, nil) for graceful shutdown after successful connection,
// (true, error) if connection was established but later failed,
// (false, error) if connection could not be established.
func (a *Amqp) consumeWithConnection() (connectedSuccessfully bool, err error) {
	a.log.Infof("Starting AMQP consumer for queue: %s", a.Cfg.Queue)

	conn, err := a.dialer()
	if err != nil {
		return false, fmt.Errorf("failed to instantiate a connection: %w", err)
	}
	defer conn.Close()
	a.log.Debug("AMQP connection established")

	ch, err := conn.Channel()
	if err != nil {
		return false, fmt.Errorf("failed to instantiate a channel: %w", err)
	}
	defer ch.Close()
	a.log.Debug("AMQP channel created")

	// Set up close notification channels for proactive failure detection
	connCloseChan := conn.NotifyClose(make(chan *amqp.Error, 1))
	chCloseChan := ch.NotifyClose(make(chan *amqp.Error, 1))

	queueArgs := amqp.Table{}
	if a.Cfg.DeadLetterExchange != nil {
		queueArgs[DeadLetterExchangeHeader] = *a.Cfg.DeadLetterExchange
	}

	q, err := ch.QueueDeclare(
		a.Cfg.Queue, // name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		queueArgs,   // arguments
	)
	if err != nil {
		return false, fmt.Errorf("failed to declare queue: %w", err)
	}

	if a.Cfg.BoundExchange != nil && *a.Cfg.BoundExchange != "" {
		err := ch.QueueBind(q.Name, a.Cfg.RoutingKey, *a.Cfg.BoundExchange, false, nil)
		if err != nil {
			return false, fmt.Errorf("failed to bind queue to exchange: %w", err)
		}
	}

	messages, err := ch.ConsumeWithContext(
		a.ctx,
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return false, fmt.Errorf("failed to consume messages: %w", err)
	}
	a.log.Infof("AMQP consumer started, waiting for messages on queue: %s", q.Name)

	mm := metrics.GetDPInstance(a.licenser)
	mm.IncrementIngestTotal(a.source.UID, a.source.ProjectID)

	a.log.Debug("Entering AMQP message processing loop")
	for {
		select {
		case <-a.ctx.Done():
			a.log.Info("AMQP consumer stopping due to context cancellation")
			return true, nil
		case amqpErr := <-connCloseChan:
			if amqpErr != nil {
				return true, fmt.Errorf("AMQP connection closed: %w", amqpErr)
			}
			return true, fmt.Errorf("AMQP connection closed unexpectedly")
		case amqpErr := <-chCloseChan:
			if amqpErr != nil {
				return true, fmt.Errorf("AMQP channel closed: %w", amqpErr)
			}
			return true, fmt.Errorf("AMQP channel closed unexpectedly")
		case d, ok := <-messages:
			if !ok {
				return true, fmt.Errorf("AMQP messages channel closed")
			}
			a.processMessage(d, mm)
		}
	}
}

// processMessage handles individual message processing including
// header marshaling, handler invocation, and acknowledgment.
func (a *Amqp) processMessage(d amqp.Delivery, mm *metrics.Metrics) {
	a.log.Debugf("AMQP message received, body length: %d bytes", len(d.Body))

	if d.Headers == nil {
		d.Headers = amqp.Table{}
	}
	d.Headers[common.BrokerMessageHeader] = d.MessageId

	headers, err := msgpack.EncodeMsgPack(d.Headers)
	if err != nil {
		a.log.WithError(err).Error("failed to marshall message headers")
	}

	a.log.Debugf("Processing AMQP message: %s", string(d.Body))
	if err := a.handler(a.ctx, a.source, string(d.Body), headers); err != nil {
		a.log.WithError(err).Error("failed to write message to create event queue - amqp pub sub")
		// Reject the message and send it to DLQ
		if err := d.Nack(false, false); err != nil {
			a.log.WithError(err).Error("failed to nack message")
			mm.IncrementIngestErrorsTotal(a.source)
		}
	} else {
		a.log.Debug("AMQP message processed successfully")
		// Acknowledge successful processing
		if err := d.Ack(false); err != nil {
			a.log.WithError(err).Error("failed to ack message")
			mm.IncrementIngestErrorsTotal(a.source)
		} else {
			a.log.Debug("AMQP message acknowledged")
			mm.IncrementIngestConsumedTotal(a.source)
		}
	}
}

// calculateBackoff returns the delay duration for a reconnection attempt
// using exponential backoff with jitter, capped at maxReconnectDelay.
func (a *Amqp) calculateBackoff(attempt uint64) time.Duration {
	// Calculate base delay with exponential backoff
	backoff := float64(initialReconnectDelay) * math.Pow(reconnectMultiplier, float64(attempt-1))

	// Cap at maximum delay
	if backoff > float64(maxReconnectDelay) {
		backoff = float64(maxReconnectDelay)
	}

	// Add jitter (0-25% of the backoff)
	jitter := backoff * 0.25 * rand.Float64()

	return time.Duration(backoff + jitter)
}

// handleError recovers from panics in the consumer goroutine
// and logs the error for debugging.
func (a *Amqp) handleError() {
	if err := recover(); err != nil {
		a.log.WithError(fmt.Errorf("sourceID: %s, Error: %v", a.source.UID, err)).Error("amqp pubsub source crashed")
	}
}
