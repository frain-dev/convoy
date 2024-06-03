package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/hibiken/asynq"
)

func TestExpireSecret(t *testing.T) {
	tests := []struct {
		name    string
		args    args
		wantErr error
		payload *Payload
		dbFn    func(*mocks.MockEndpointRepository)
	}{
		{
			name: "should_expire_secret",
			payload: &Payload{
				EndpointID: "abc",
				SecretID:   "secret_1",
				ProjectID:  "project_1",
			},
			dbFn: func(a *mocks.MockEndpointRepository) {
				a.EXPECT().FindEndpointByID(gomock.Any(), "abc", gomock.Any()).Times(1).Return(
					&datastore.Endpoint{
						UID:       "123",
						ProjectID: "project_1",
						Secrets: []datastore.Secret{
							{UID: "secret_1"},
						},
					},
					nil,
				)

				a.EXPECT().DeleteSecret(gomock.Any(), &datastore.Endpoint{
					UID:       "123",
					ProjectID: "project_1",
					Secrets: []datastore.Secret{
						{UID: "secret_1"},
					},
				}, "secret_1", "project_1")
			},
			wantErr: nil,
		},

		{
			name: "should_fail_to_find_endpoint",
			payload: &Payload{
				EndpointID: "abc",
				SecretID:   "secret_1",
			},
			dbFn: func(a *mocks.MockEndpointRepository) {
				a.EXPECT().FindEndpointByID(gomock.Any(), "abc", gomock.Any()).Times(1).Return(
					nil,
					datastore.ErrEndpointNotFound,
				)
			},
			wantErr: &EndpointError{Err: datastore.ErrEndpointNotFound, delay: defaultDelay},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			endpointRepo := mocks.NewMockEndpointRepository(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(endpointRepo)
			}

			fn := ExpireSecret(endpointRepo)
			buf, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			task := asynq.NewTask(string(convoy.ExpireSecretsProcessor), buf, asynq.Queue(string(convoy.DefaultQueue)), asynq.ProcessIn(time.Nanosecond))
			err = fn(context.Background(), task)

			require.Equal(t, err, tt.wantErr)
		})
	}
}
