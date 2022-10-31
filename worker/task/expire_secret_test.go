package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/hibiken/asynq"
)

func TestExpireSecret(t *testing.T) {
	tests := []struct {
		name    string
		args    args
		wantErr error
		payload *Payload
		dbFn    func(*mocks.MockApplicationRepository)
	}{
		{
			name: "should_expire_secret",
			payload: &Payload{
				AppID:      "123",
				EndpointID: "abc",
				SecretID:   "secret_1",
			},
			dbFn: func(a *mocks.MockApplicationRepository) {
				a.EXPECT().FindApplicationByID(gomock.Any(), "123").Times(1).Return(
					&datastore.Application{
						UID:     "123",
						GroupID: "group_1",
						Endpoints: []datastore.Endpoint{
							{
								UID: "abc",
								Secrets: []datastore.Secret{
									{UID: "secret_1"},
								},
							},
						},
					},
					nil,
				)

				a.EXPECT().UpdateApplication(gomock.Any(), gomock.Any(), "group_1").Times(1).Return(nil)
			},
			wantErr: nil,
		},
		{
			name: "should_fail_to_find_app",
			payload: &Payload{
				AppID:      "123",
				EndpointID: "abc",
				SecretID:   "secret_1",
			},
			dbFn: func(a *mocks.MockApplicationRepository) {
				a.EXPECT().FindApplicationByID(gomock.Any(), "123").Times(1).Return(
					nil,
					errors.New("failed"),
				)
			},
			wantErr: &EndpointError{Err: errors.New("failed"), delay: defaultDelay},
		},
		{
			name: "should_fail_to_find_endpoint",
			payload: &Payload{
				AppID:      "123",
				EndpointID: "abc",
				SecretID:   "secret_1",
			},
			dbFn: func(a *mocks.MockApplicationRepository) {
				a.EXPECT().FindApplicationByID(gomock.Any(), "123").Times(1).Return(
					&datastore.Application{
						UID:     "123",
						GroupID: "group_1",
						Endpoints: []datastore.Endpoint{
							{UID: "abcdd"},
						},
					},
					nil,
				)
			},
			wantErr: &EndpointError{Err: datastore.ErrEndpointNotFound, delay: defaultDelay},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			appRepo := mocks.NewMockApplicationRepository(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(appRepo)
			}

			fn := ExpireSecret(appRepo)
			buf, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			task := asynq.NewTask(string(convoy.ExpireSecretsProcessor), buf, asynq.Queue(string(convoy.DefaultQueue)), asynq.ProcessIn(time.Nanosecond))
			err = fn(context.Background(), task)

			require.Equal(t, err, tt.wantErr)
		})
	}
}
