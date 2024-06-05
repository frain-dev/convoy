package task

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func Test_ProcessEmails(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
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
			expectedError: ErrInvalidEmailPayload,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange.
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			sc := mocks.NewMockSmtpClient(ctrl)
			tc.clientFn(sc)

			buf := []byte(tc.payload)
			job := &queue.Job{
				Payload: json.RawMessage(buf),
				Delay:   0,
			}

			task := asynq.NewTask(
				string(convoy.EmailProcessor),
				job.Payload,
				asynq.Queue(string(convoy.DefaultQueue)),
				asynq.ProcessIn(job.Delay))

			processFn := ProcessEmails(sc)

			// Act.
			err := processFn(context.Background(), task)

			// Assert.
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
