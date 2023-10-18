package services

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideExpireSecretService(ctrl *gomock.Controller, S *models.ExpireSecret, Endpoint *datastore.Endpoint, Project *datastore.Project) *ExpireSecretService {
	return &ExpireSecretService{
		Queuer:       mocks.NewMockQueuer(ctrl),
		Cache:        mocks.NewMockCache(ctrl),
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		ProjectRepo:  mocks.NewMockProjectRepository(ctrl),
		S:            S,
		Endpoint:     Endpoint,
		Project:      Project,
	}
}

func TestExpireSecretService_Run(t *testing.T) {
	ctx := context.Background()
	project := &datastore.Project{UID: "1234567890"}
	type args struct {
		ctx      context.Context
		secret   *models.ExpireSecret
		endpoint *datastore.Endpoint
		project  *datastore.Project
	}
	tests := []struct {
		name       string
		args       args
		dbFn       func(es *ExpireSecretService)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_expire_endpoint_secret",
			args: args{
				ctx: ctx,
				secret: &models.ExpireSecret{
					Secret:     "abce",
					Expiration: 10,
				},
				project: project,
				endpoint: &datastore.Endpoint{
					UID:       "abc",
					ProjectID: "1234",
					Secrets: []datastore.Secret{
						{
							UID:   "1234",
							Value: "test_secret",
						},
					},
					AdvancedSignatures: false,
				},
			},
			dbFn: func(es *ExpireSecretService) {
				endpointRepo := es.EndpointRepo.(*mocks.MockEndpointRepository)

				endpointRepo.EXPECT().UpdateSecrets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				eq, _ := es.Queuer.(*mocks.MockQueuer)
				eq.EXPECT().Write(gomock.Any(), convoy.ExpireSecretsProcessor, convoy.DefaultQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			wantErr:    false,
			wantErrMsg: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideExpireSecretService(ctrl, tt.args.secret, tt.args.endpoint, tt.args.project)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(as)
			}

			_, err := as.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
