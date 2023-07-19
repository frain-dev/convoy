package pubsub

import (
	"errors"
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

type KafkaPubSub struct {
	Brokers         []string   `json:"brokers" valid:"required"`
	ConsumerGroupID string     `json:"consumer_group_id" valid:"required"`
	TopicName       string     `json:"topic_name" valid:"required"`
	Auth            *KafkaAuth `json:"auth"`
}

type KafkaAuth struct {
	Type     string `json:"type" valid:"optional,in(plain|scram)"`
	Username string `json:"username"`
	Password string `json:"password"`
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

	case datastore.KafkaPubSub:
		if cfg.KafKa == nil {
			return errors.New("kafka config is required")
		}

		kPubSub := &KafkaPubSub{
			Brokers:         cfg.KafKa.Brokers,
			ConsumerGroupID: cfg.KafKa.ConsumerGroupID,
			TopicName:       cfg.KafKa.TopicName,
		}

		if cfg.KafKa.Auth != nil {
			kPubSub.Auth = &KafkaAuth{
				Type:     cfg.KafKa.Auth.Type,
				Username: cfg.KafKa.Auth.Username,
				Password: cfg.KafKa.Auth.Password,
			}
		}

		if err := util.Validate(kPubSub); err != nil {
			return err
		}

		k := &kafka.Kafka{Cfg: cfg.KafKa}
		if err := k.Verify(); err != nil {
			return err
		}

		return nil

	default:
		return nil
	}
}
