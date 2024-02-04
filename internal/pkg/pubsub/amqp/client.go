package rqm

import (
	"context"
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	amqp "github.com/rabbitmq/amqp091-go"
)

var ErrInvalidCredentials = errors.New("your kafka credentials are invalid. please verify you're providing the correct credentials")

type Amqp struct {
	Cfg     *datastore.AmqpPubSubConfig
	source  *datastore.Source
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
	handler datastore.PubSubHandler
	log     log.StdLogger
}

func New(source *datastore.Source, handler datastore.PubSubHandler, log log.StdLogger) *Amqp {
	ctx, cancel := context.WithCancel(context.Background())

	return &Amqp{
		Cfg:     source.PubSub.Amqp,
		source:  source,
		workers: source.PubSub.Workers,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
		log:     log,
	}
}

func (k *Amqp) Start() {
	for i := 1; i <= k.workers; i++ {
		go k.Consume()
	}
}

func (k *Amqp) dialer() (*amqp.Connection, error) {
	auth := ""
	if k.Cfg.Auth != nil {
		auth = fmt.Sprintf("%s:%s@", k.Cfg.Auth.User, k.Cfg.Auth.Password)
	}
	connString := fmt.Sprintf("%s://%s%s:%s/", k.Cfg.Schema, auth, k.Cfg.Host, k.Cfg.Port)
	conn, err := amqp.Dial(connString)
	if err != nil {
		log.WithError(err).Error("Failed to open connection to amqp")
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
		log.WithError(err).Error("failed to instanciate a channel")
		return err
	}
	defer ch.Close()

	return nil

}

func (k *Amqp) Consume() {
	conn, err := k.dialer()
	if err != nil {
		log.WithError(err).Error("failed to instanciate a connection")
		return
	}

	ch, err := conn.Channel()
	if err != nil {
		log.WithError(err).Error("failed to instanciate a channel")
		return
	}

	q, err := ch.QueueDeclare(
		k.Cfg.Queue, // name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)

	if k.Cfg.BindedExchange != nil && *k.Cfg.BindedExchange != "" {
		ch.QueueBind(q.Name, k.Cfg.RoutingKey, *k.Cfg.BindedExchange, false, nil)
	}

	if err != nil {
		log.WithError(err).Error("failed to declare queue")
		return
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)

	if err != nil {
		log.WithError(err).Error("failed to consume messages")
		return
	}

	var forever chan struct{}

	for d := range msgs {
		ctx := context.Background()

		if err := k.handler(ctx, k.source, string(d.Body)); err != nil {
			k.log.WithError(err).Error("failed to write message to create event queue - amqp pub sub")
		}
	}

	<-forever

	return
}

func (k *Amqp) Stop() {
	k.cancel()
}
