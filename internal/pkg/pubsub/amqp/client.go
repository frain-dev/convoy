package rqm

import (
	"context"
	"fmt"

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

func (k *Amqp) Start(ctx context.Context) {
	k.ctx = ctx
	for i := 1; i <= k.workers; i++ {
		go k.consume()
	}
}

func (k *Amqp) dialer() (*amqp.Connection, error) {
	auth := ""
	if k.Cfg.Auth != nil {
		auth = fmt.Sprintf("%s:%s@", k.Cfg.Auth.User, k.Cfg.Auth.Password)
	}

	connString := fmt.Sprintf("%s://%s%s:%s/%s?heartbeat=30", k.Cfg.Schema, auth, k.Cfg.Host, k.Cfg.Port, *k.Cfg.Vhost)
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

func (k *Amqp) Verify() error {
	conn, err := k.dialer()
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

func (k *Amqp) consume() {
	fmt.Printf("[AMQP] consume() called for queue: %s\n", k.Cfg.Queue)
	conn, err := k.dialer()
	if err != nil {
		fmt.Printf("[AMQP] ERROR: failed to instantiate a connection: %v\n", err)
		log.WithError(err).Error("failed to instantiate a connection")
		return
	}
	fmt.Printf("[AMQP] Connection established\n")

	ch, err := conn.Channel()
	if err != nil {
		fmt.Printf("[AMQP] ERROR: failed to instantiate a channel: %v\n", err)
		log.WithError(err).Error("failed to instantiate a channel")
		return
	}
	fmt.Printf("[AMQP] Channel created\n")

	defer conn.Close()
	defer ch.Close()

	queueArgs := amqp.Table{}
	if k.Cfg.DeadLetterExchange != nil {
		queueArgs[DeadLetterExchangeHeader] = *k.Cfg.DeadLetterExchange
	}

	q, err := ch.QueueDeclare(
		k.Cfg.Queue, // name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		queueArgs,   // arguments
	)

	if k.Cfg.BoundExchange != nil && *k.Cfg.BoundExchange != "" {
		err := ch.QueueBind(q.Name, k.Cfg.RoutingKey, *k.Cfg.BoundExchange, false, nil)
		if err != nil {
			log.WithError(err).Error("failed to bind queue to exchange")
			return
		}
	}

	if err != nil {
		log.WithError(err).Error("failed to declare queue")
		return
	}

	messages, err := ch.ConsumeWithContext(
		k.ctx,
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		fmt.Printf("[AMQP] ERROR: failed to consume messages: %v\n", err)
		log.WithError(err).Error("failed to consume messages")
		return
	}
	fmt.Printf("[AMQP] ConsumeWithContext() success, waiting for messages on queue: %s\n", q.Name)

	mm := metrics.GetDPInstance(k.licenser)
	mm.IncrementIngestTotal(k.source.UID, k.source.ProjectID)

	fmt.Printf("[AMQP] Entering message loop\n")
	for d := range messages {
		fmt.Printf("[AMQP] Message received! Body length: %d\n", len(d.Body))
		if d.Headers == nil {
			d.Headers = amqp.Table{}
		}
		d.Headers[common.BrokerMessageHeader] = d.MessageId

		headers, err := msgpack.EncodeMsgPack(d.Headers)
		if err != nil {
			fmt.Printf("[AMQP] ERROR: failed to marshall message headers: %v\n", err)
			k.log.WithError(err).Error("failed to marshall message headers")
		}

		fmt.Printf("[AMQP] Calling handler for message, body: %s\n", string(d.Body))
		if err := k.handler(k.ctx, k.source, string(d.Body), headers); err != nil {
			fmt.Printf("[AMQP] Handler returned error: %v\n", err)
			k.log.WithError(err).Error("failed to write message to create event queue - amqp pub sub")
			// Reject the message and send it to DLQ
			if err := d.Nack(false, false); err != nil {
				k.log.WithError(err).Error("failed to nack message")
				mm.IncrementIngestErrorsTotal(k.source)
			}
		} else {
			fmt.Printf("[AMQP] Handler succeeded\n")
			// Acknowledge successful processing
			if err := d.Ack(false); err != nil {
				fmt.Printf("[AMQP] ERROR: failed to ack message: %v\n", err)
				k.log.WithError(err).Error("failed to ack message")
				mm.IncrementIngestErrorsTotal(k.source)
			} else {
				fmt.Printf("[AMQP] Message ACK'd successfully\n")
				mm.IncrementIngestConsumedTotal(k.source)
			}
		}
	}
}
