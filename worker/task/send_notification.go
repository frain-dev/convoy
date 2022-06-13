package task

import (
	"context"
	"encoding/json"
	"github.com/frain-dev/convoy/notification"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

func SendNotification(notificationSender notification.Sender) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		if notificationSender == nil {
			return nil
		}

		buf := t.Payload()

		n := &notification.Notification{}
		err := json.Unmarshal(buf, n)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal notification payload")
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		err = notificationSender.SendNotification(ctx, n)
		if err != nil {
			log.WithError(err).Error("failed to send email notification")
			return &EndpointError{Err: err, delay: defaultDelay}
		}
		return nil
	}
}
