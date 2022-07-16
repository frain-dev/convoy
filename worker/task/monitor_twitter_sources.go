package task

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/notification"
	"github.com/frain-dev/convoy/notification/email"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

func MonitorTwitterSources(sourceRepo datastore.SourceRepository, subRepo datastore.SubscriptionRepository, appRepo datastore.ApplicationRepository, queue queue.Queuer) func(context.Context, *asynq.Task) error {
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
				//the crc verified at timestamp must not be less than two hours ago
				if crcExpiry.After(source.ProviderConfig.Twitter.CrcVerifiedAt.Time()) {
					subscriptions, err := subRepo.FindSubscriptionsBySourceIDs(ctx, source.GroupID, source.UID)
					if err != nil {
						log.Error("Failed to load sources paged")
						return err
					}

					for _, s := range subscriptions {
						app, err := appRepo.FindApplicationByID(ctx, s.AppID)
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

func sendNotificationEmail(source datastore.Source, app *datastore.Application, q queue.Queuer) error {
	n := &notification.Notification{
		Email:             app.SupportEmail,
		EmailTemplateName: email.TemplateTwitterSource.String(),
		SourceName:        source.Name,
		CrcVerifiedAt:     source.ProviderConfig.Twitter.CrcVerifiedAt.Time().String(),
		Subject:           "Twitter Custom Source",
	}

	buf, err := json.Marshal(n)
	if err != nil {
		log.WithError(err).Error("failed to marshal notification payload")
		return err
	}

	job := &queue.Job{
		Payload: json.RawMessage(buf),
		Delay:   0,
	}

	err = q.Write(convoy.NotificationProcessor, convoy.ScheduleQueue, job)
	if err != nil {
		log.WithError(err).Error("failed to write new notification to the queue")
		return err
	}
	return nil
}
