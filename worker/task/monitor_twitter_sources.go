package task

import (
	"context"
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy/pkg/log"
	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

func MonitorTwitterSources(store datastore.Store, queue queue.Queuer) func(context.Context, *asynq.Task) error {
	sourceRepo := mongo.NewSourceRepo(store)
	subRepo := mongo.NewSubscriptionRepo(store)
	endpointRepo := mongo.NewEndpointRepo(store)

	return func(ctx context.Context, t *asynq.Task) error {
		p := datastore.Pageable{Page: 1, PerPage: 100}
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
			if now.After(source.CreatedAt.Time().Add(time.Hour)) {
				// the crc verified at timestamp must not be less than two hours ago
				if crcExpiry.After(source.ProviderConfig.Twitter.CrcVerifiedAt.Time()) {
					subscriptions, err := subRepo.FindSubscriptionsBySourceIDs(ctx, source.ProjectID, source.UID)
					if err != nil {
						log.Error("Failed to load sources paged")
						return err
					}

					for _, s := range subscriptions {
						app, err := endpointRepo.FindEndpointByID(ctx, s.EndpointID)
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
			"crc_verified_at": source.ProviderConfig.Twitter.CrcVerifiedAt.Time().String(),
			"source_name":     source.Name,
		},
	}

	buf, err := json.Marshal(em)
	if err != nil {
		log.WithError(err).Error("failed to marshal notification payload")
		return err
	}

	job := &queue.Job{
		Payload: json.RawMessage(buf),
		Delay:   0,
	}

	err = q.Write(convoy.NotificationProcessor, convoy.DefaultQueue, job)
	if err != nil {
		log.WithError(err).Error("failed to write new notification to the queue")
		return err
	}
	return nil
}
