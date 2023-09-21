package task

import (
	"context"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

func MonitorTwitterSources(db database.Database, queue queue.Queuer) func(context.Context, *asynq.Task) error {
	sourceRepo := postgres.NewSourceRepo(db)
	subRepo := postgres.NewSubscriptionRepo(db)
	endpointRepo := postgres.NewEndpointRepo(db)

	return func(ctx context.Context, t *asynq.Task) error {
		p := datastore.Pageable{PerPage: 100, Direction: datastore.Next, NextCursor: datastore.DefaultCursor}
		f := &datastore.SourceFilter{Provider: string(datastore.TwitterSourceProvider)}

		sources, _, err := sourceRepo.LoadSourcesPaged(context.Background(), "", f, p)
		if err != nil {
			log.Error("Failed to load sources paged")
			return err
		}

		for _, source := range sources {
			now := time.Now()
			crcExpiry := time.Now().Add(time.Hour * -2)

			// the source needs to have been created at least one hour ago
			if now.After(source.CreatedAt.Add(time.Hour)) {
				expiry := source.ProviderConfig.Twitter.CrcVerifiedAt.Time
				// the crc verified at timestamp must not be less than two hours ago
				if crcExpiry.After(expiry) {
					subscriptions, err := subRepo.FindSubscriptionsBySourceID(ctx, source.ProjectID, source.UID)
					if err != nil {
						log.Error("Failed to load sources paged")
						return err
					}

					for _, s := range subscriptions {
						app, err := endpointRepo.FindEndpointByID(ctx, s.EndpointID, s.ProjectID)
						if err != nil {
							log.Error("Failed to load sources paged")
							return err
						}

						if !util.IsStringEmpty(app.SupportEmail) {
							err = sendNotificationEmail(source, app, queue)
							if err != nil {
								log.Error("failed to send notification")
								return err
							}
						}
					}
				}
			}
		}
		return nil
	}
}

func sendNotificationEmail(source datastore.Source, endpoint *datastore.Endpoint, q queue.Queuer) error {
	em := email.Message{
		Email:        endpoint.SupportEmail,
		Subject:      "Twitter Custom Source",
		TemplateName: email.TemplateTwitterSource,
		Params: map[string]string{
			"crc_verified_at": source.ProviderConfig.Twitter.CrcVerifiedAt.Time.String(),
			"source_name":     source.Name,
		},
	}

	bytes, err := msgpack.EncodeMsgPack(em)
	if err != nil {
		log.WithError(err).Error("failed to marshal notification payload")
		return err
	}

	job := &queue.Job{
		Payload: bytes,
		Delay:   0,
	}

	err = q.Write(convoy.NotificationProcessor, convoy.DefaultQueue, job)
	if err != nil {
		log.WithError(err).Error("failed to write new notification to the queue")
		return err
	}
	return nil
}
