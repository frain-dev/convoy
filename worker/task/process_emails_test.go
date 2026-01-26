package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	jobenvelope "github.com/olamilekan000/surge/surge/job"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
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

			jobEnvelope := &jobenvelope.JobEnvelope{
				ID:        "",
				Topic:     string(convoy.EmailProcessor),
				Args:      job.Payload,
				Namespace: "default",
				Queue:     string(convoy.DefaultQueue),
				State:     jobenvelope.StatePending,
				CreatedAt: time.Now(),
			}

			processFn := ProcessEmails(sc)

			// Act.
			err := processFn(context.Background(), jobEnvelope)

			// Assert.
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
