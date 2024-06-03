package task

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestProcessNotifications(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		nFn           func() func()
		clientFn      func(sc *mocks.MockSmtpClient)
		expectedError error
	}{
		{
			name:    "should_fail_for_invalid_payload",
			payload: `bad payload`,
			clientFn: func(sc *mocks.MockSmtpClient) {
				sc.EXPECT().
					SendEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(0)
			},
			expectedError: ErrInvalidNotificationPayload,
		},
		{
			name: "should_fail_for_invalid_notification_type",
			payload: `
				{
					"notification_type": "invalid"
				}
			`,
			clientFn:      nil,
			expectedError: ErrInvalidNotificationType,
		},
		{
			name: "should_fail_for_invalid_email_payload",
			payload: `
				{
					"notification_type": "email",
					"payload": "invalid"
				}
			`,
			clientFn:      nil,
			expectedError: ErrInvalidEmailPayload,
		},
		{
			name: "should_pass_for_valid_email_notification",
			payload: `
				{
					"notification_type": "email",
					"payload": {
						"email": "user@default.com",
						"subject": "organisation invite",
						"template_name": "organisation.invite"
					}
				}
			`,
			clientFn: func(sc *mocks.MockSmtpClient) {
				sc.EXPECT().
					SendEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
			},
			expectedError: nil,
		},
		{
			name: "should_fail_for_invalid_slack_payload",
			payload: `
				{
					"notification_type": "slack",
					"payload": "invalid"
				}
			`,
			clientFn:      nil,
			expectedError: ErrInvalidSlackPayload,
		},
		{
			name: "should_fail_for_valid_slack_message",
			payload: `
				{
					"notification_type": "slack",
					"payload": {
						"webhook_url": "https://hooks.slack.com/services/T00/B00/X",
						"text": "endpoint re-activated"
					}
				}
			`,
			nFn: func() func() {
				httpmock.Activate()

				url := "https://hooks.slack.com/services/T00/B00/X"

				httpmock.RegisterResponder(http.MethodPost, url,
					httpmock.NewStringResponder(http.StatusOK, string("200 Ok!")))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
			clientFn:      nil,
			expectedError: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange.
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			sc := mocks.NewMockSmtpClient(ctrl)
			if tc.clientFn != nil {
				tc.clientFn(sc)
			}

			if tc.nFn != nil {
				deferFn := tc.nFn()
				defer deferFn()
			}

			buf := []byte(strings.TrimSpace(tc.payload))
			job := &queue.Job{
				Payload: json.RawMessage(buf),
				Delay:   0,
			}

			task := asynq.NewTask(
				string(convoy.NotificationProcessor),
				job.Payload,
				asynq.Queue(string(convoy.DefaultQueue)),
				asynq.ProcessIn(job.Delay))

			processFn := ProcessNotifications(sc)

			// Act.
			err := processFn(context.Background(), task)

			// Assert.
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
