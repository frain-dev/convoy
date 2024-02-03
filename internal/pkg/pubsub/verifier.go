package pubsub

import (
	"errors"

	rqm "github.com/frain-dev/convoy/internal/pkg/pubsub/amqp"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/kafka"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/google"
	"github.com/frain-dev/convoy/internal/pkg/pubsub/sqs"
	"github.com/frain-dev/convoy/util"
)

type GooglePubSub struct {
	ServiceAccount []byte `json:"service_account" valid:"required~service account is required"`
	SubscriptionID string `json:"subscription_id" valid:"required~subscription id is required"`
	ProjectID      string `json:"project_id" valid:"required~project id is required"`
}

type SqsPubSub struct {
	AccessKeyID   string `json:"access_key_id" valid:"required"`
	SecretKey     string `json:"secret_key" valid:"required"`
	DefaultRegion string `json:"default_region" valid:"required"`
	QueueName     string `json:"queue_name" valid:"required"`
}

type AmqpPubSub struct {
	Host           string    `json:"host" valid:"amqp host is required"`
	Port           string    `json:"port" valid:"amqp port is required"`
	Queue          string    `json:"queue" valid:"amqp queue name is required"`
	Schema         string    `json:"schema" valid:"amqp schema is required"`
	Auth           *AmqpAuth `json:"auth"`
	BindedExchange *string   `json:"bindedExchange"`
	RoutingKey     string    `json:"routingKey"`
}

type AmqpAuth struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

type KafkaPubSub struct {
	Brokers         []string   `json:"brokers" valid:"required~brokers list is required"`
	ConsumerGroupID string     `json:"consumer_group_id"`
	TopicName       string     `json:"topic_name" valid:"required~topic name is required"`
	Auth            *KafkaAuth `json:"auth"`
}

type KafkaAuth struct {
	Type     string `json:"type" valid:"optional,in(plain|scram)~unsupported auth type"`
	Hash     string `json:"hash" valid:"optional,in(SHA256|SHA512)~unsupported hashing algorithm"`
	Username string `json:"username"`
	Password string `json:"password"`
	TLS      bool   `json:"tls"`
}

type PS struct {
	Type    datastore.PubSubType `json:"type" valid:"required~type is required,supported_pub_sub~unsupported pub sub type"`
	Workers int                  `json:"workers" valid:"required"`
}

func Validate(cfg *datastore.PubSubConfig) error {
	ps := struct {
		PubSub PS `json:"pub_sub" valid:"required"`
	}{
		PubSub: PS{
			Type:    cfg.Type,
			Workers: cfg.Workers,
		},
	}

	err := util.Validate(ps)
	if err != nil {
		return err
	}

	switch cfg.Type {
	case datastore.GooglePubSub:
		if cfg.Google == nil {
			return errors.New("google pub sub config is required")
		}

		gPubSub := &GooglePubSub{
			ServiceAccount: cfg.Google.ServiceAccount,
			SubscriptionID: cfg.Google.SubscriptionID,
			ProjectID:      cfg.Google.ProjectID,
		}

		if err := util.Validate(gPubSub); err != nil {
			return err
		}

		g := &google.Google{Cfg: cfg.Google}
		if err := g.Verify(); err != nil {
			return err
		}

		return nil

	case datastore.SqsPubSub:
		if cfg.Sqs == nil {
			return errors.New("sqs config is required")
		}

		sPubSub := &SqsPubSub{
			AccessKeyID:   cfg.Sqs.AccessKeyID,
			SecretKey:     cfg.Sqs.SecretKey,
			DefaultRegion: cfg.Sqs.DefaultRegion,
			QueueName:     cfg.Sqs.QueueName,
		}

		if err := util.Validate(sPubSub); err != nil {
			return err
		}

		s := &sqs.Sqs{Cfg: cfg.Sqs}
		if err := s.Verify(); err != nil {
			return err
		}

		return nil

	case datastore.AmqpPubSub:
		if cfg.Amqp == nil {
			return errors.New("amqp config is required")
		}

		var aAuth *AmqpAuth
		if cfg.Amqp.Auth != nil {
			aAuth = &AmqpAuth{
				User:     cfg.Amqp.Auth.User,
				Password: cfg.Amqp.Auth.Password,
			}
		}

		aPubSub := &AmqpPubSub{
			Schema:         cfg.Amqp.Schema,
			Host:           cfg.Amqp.Host,
			Port:           cfg.Amqp.Port,
			Queue:          cfg.Amqp.Queue,
			Auth:           aAuth,
			BindedExchange: cfg.Amqp.BindedExchange,
			RoutingKey:     cfg.Amqp.RoutingKey,
		}

		if err := util.Validate(aPubSub); err != nil {
			return err
		}

		a := &rqm.Amqp{Cfg: cfg.Amqp}
		if err := a.Verify(); err != nil {
			return err
		}

		return nil

	case datastore.KafkaPubSub:
		if cfg.Kafka == nil {
			return errors.New("kafka config is required")
		}

		kPubSub := &KafkaPubSub{
			Brokers:         cfg.Kafka.Brokers,
			ConsumerGroupID: cfg.Kafka.ConsumerGroupID,
			TopicName:       cfg.Kafka.TopicName,
		}

		if cfg.Kafka.Auth != nil {
			kPubSub.Auth = &KafkaAuth{
				Type:     cfg.Kafka.Auth.Type,
				Hash:     cfg.Kafka.Auth.Hash,
				Username: cfg.Kafka.Auth.Username,
				Password: cfg.Kafka.Auth.Password,
				TLS:      cfg.Kafka.Auth.TLS,
			}
		}

		if err := util.Validate(kPubSub); err != nil {
			return err
		}

		k := &kafka.Kafka{Cfg: cfg.Kafka}
		if err := k.Verify(); err != nil {
			return err
		}

		return nil

	default:
		return nil
	}
}
