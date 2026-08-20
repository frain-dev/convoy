package task

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/internal/endpoints"
	"github.com/frain-dev/convoy/internal/sources"
	"github.com/frain-dev/convoy/internal/subscriptions"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

func MonitorTwitterSources(db database.Database, queue queue.Queuer, locker JobLocker, logger log.Logger) func(context.Context, *asynq.Task) error {
	sourceRepo := sources.New(logger, db)
	subRepo := subscriptions.New(logger, db)
	endpointRepo := endpoints.New(logger, db)

	return func(ctx context.Context, t *asynq.Task) error {
		// One page of at most 100 twitter sources, with per-source subscription
		// and endpoint lookups plus notification enqueues; 5m is ample.
		return locker.WithLock(ctx, "convoy:monitor_twitter_sources:mutex", 5*time.Minute, func(ctx context.Context) error {
			p := datastore.Pageable{PerPage: 100, Direction: datastore.Next, NextCursor: datastore.DefaultCursor}
			f := &datastore.SourceFilter{Provider: string(datastore.TwitterSourceProvider)}

			sources, _, err := sourceRepo.LoadSourcesPaged(ctx, "", f, p)
			if err != nil {
				logger.Error("Failed to load sources paged")
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
							logger.Error("Failed to load sources paged")
							return err
						}

						for _, s := range subscriptions {
							app, err := endpointRepo.FindEndpointByID(ctx, s.EndpointID, s.ProjectID)
							if err != nil {
								logger.Error("Failed to load sources paged")
								return err
							}

							if !util.IsStringEmpty(app.SupportEmail) {
								err = sendNotificationEmail(ctx, source, app, queue, logger)
								if err != nil {
									logger.Error("failed to send notification")
									return err
								}
							}
						}
					}
				}
			}
			return nil
		})
	}
}

func sendNotificationEmail(ctx context.Context, source datastore.Source, endpoint *datastore.Endpoint, q queue.Queuer, logger log.Logger) error {
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
		logger.Error("failed to marshal notification payload", "error", err)
		return err
	}

	job := &queue.Job{
		ID:      ulid.Make().String(),
		Payload: bytes,
	}

	err = q.Write(ctx, convoy.NotificationProcessor, convoy.DefaultQueue, job)
	if err != nil {
		logger.Error("failed to write new notification to the queue", "error", err)
		return err
	}
	return nil
}
