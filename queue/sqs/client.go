package sqs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"

	"strings"
)

type client struct {
	inner             *sqs.SQS
	queueUrl          string
	delaySeconds      uint16
	maxMessages       uint8
	visibilityTimeout uint16
	closeChan         chan struct{}
}

const (
	maxVisibilityTimeout = (12 * 60 * 60)
	maxDelaySeconds      = 900
)

func New(cfg config.Configuration) (queue.Queuer, error) {
	if cfg.Queue.Type != config.SqsQueueProvider {
		return nil, errors.New("please select the sqs driver in your config")
	}

	dsn := cfg.Queue.Sqs.DSN
	if util.IsStringEmpty(dsn) {
		return nil, errors.New("please provide the Sqs DSN")
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Profile:           cfg.Queue.Sqs.Profile,
	}))

	svc := sqs.New(sess)

	var delaySeconds uint16

	if cfg.Queue.Sqs.DelaySeconds > maxDelaySeconds {
		delaySeconds = maxDelaySeconds
	}

	var maxMessages uint8

	if cfg.Queue.Sqs.MaxMessages > 10 {
		maxMessages = 10
	}

	if cfg.Queue.Sqs.MaxMessages == 0 {
		maxMessages = 1
	}

	var visibilityTimeout uint16

	if cfg.Queue.Sqs.VisibilityTimeout > maxVisibilityTimeout {
		visibilityTimeout = maxVisibilityTimeout
	}

	return &client{
		inner:             svc,
		queueUrl:          cfg.Queue.Sqs.DSN,
		delaySeconds:      delaySeconds,
		maxMessages:       maxMessages,
		visibilityTimeout: visibilityTimeout,
	}, nil
}

func (c *client) Close() error {
	c.closeChan <- struct{}{}
	return nil
}

func (c *client) receiveMessage() (*sqs.ReceiveMessageOutput, error) {
	return c.inner.ReceiveMessage(&sqs.ReceiveMessageInput{
		MessageAttributeNames: []*string{
			aws.String(sqs.QueueAttributeNameAll),
		},
		QueueUrl:            aws.String(c.queueUrl),
		MaxNumberOfMessages: aws.Int64(int64(c.maxMessages)),
		VisibilityTimeout:   aws.Int64(int64(c.visibilityTimeout)),
	})
}

func (c *client) Read() chan queue.Message {
	channel := make(chan queue.Message, c.maxMessages)

	sqsReadMsg, err := c.receiveMessage()
	if err != nil {
		return channel
	}

	for _, msg := range sqsReadMsg.Messages {
		var m hookcamp.Message

		if err := json.NewDecoder(strings.NewReader(*msg.Body)).Decode(&m); err != nil {
			channel <- queue.Message{
				Err: err,
			}

			continue
		}

		channel <- queue.Message{
			Err:  nil,
			Data: m,
		}
	}

	return channel
}

func (c *client) Write(ctx context.Context, msg hookcamp.Message) error {
	b := new(bytes.Buffer)

	if err := json.NewEncoder(b).Encode(&msg); err != nil {
		return err
	}

	_, err := c.inner.SendMessage(&sqs.SendMessageInput{
		DelaySeconds: aws.Int64(int64(c.delaySeconds)),
		MessageBody:  aws.String(b.String()),
		QueueUrl:     aws.String(c.queueUrl),
	})

	return err
}
