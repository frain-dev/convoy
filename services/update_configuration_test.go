package services

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideUpdateConfigService(ctrl *gomock.Controller, config *models.Configuration) *UpdateConfigService {
	return &UpdateConfigService{
		ConfigRepo: mocks.NewMockConfigurationRepository(ctrl),
		Config:     config,
	}
}

func TestUpdateConfigService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx       context.Context
		newConfig *models.Configuration
	}

	tests := []struct {
		name       string
		args       args
		wantConfig *datastore.Configuration
		dbFn       func(c *UpdateConfigService)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_update_configuration",
			args: args{
				ctx: ctx,
				newConfig: &models.Configuration{IsAnalyticsEnabled: boolPtr(true), StoragePolicy: &models.StoragePolicyConfiguration{
					Type: datastore.OnPrem,
					OnPrem: &models.OnPremStorage{
						Path: null.NewString("/tmp/", true),
					},
				}},
			},
			wantConfig: &datastore.Configuration{IsAnalyticsEnabled: true, StoragePolicy: &datastore.StoragePolicyConfiguration{
				Type: datastore.OnPrem,
				OnPrem: &datastore.OnPremStorage{
					Path: null.NewString("/tmp/", true),
				},
			}},
			dbFn: func(c *UpdateConfigService) {
				co, _ := c.ConfigRepo.(*mocks.MockConfigurationRepository)
				co.EXPECT().LoadConfiguration(gomock.Any()).Times(1).Return(&datastore.Configuration{IsAnalyticsEnabled: true, StoragePolicy: &datastore.StoragePolicyConfiguration{
					Type: datastore.OnPrem,
					OnPrem: &datastore.OnPremStorage{
						Path: null.NewString("/tmp/", true),
					},
				}}, nil)
				co.EXPECT().UpdateConfiguration(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},
		{
			name: "should_fail_to_update_configuration",
			args: args{
				ctx:       ctx,
				newConfig: &models.Configuration{IsAnalyticsEnabled: boolPtr(true)},
			},
			dbFn: func(c *UpdateConfigService) {
				co, _ := c.ConfigRepo.(*mocks.MockConfigurationRepository)
				co.EXPECT().LoadConfiguration(gomock.Any()).Times(1).Return(nil, datastore.ErrConfigNotFound)
			},
			wantErr:    true,
			wantErrMsg: "config not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			c := provideUpdateConfigService(ctrl, tc.args.newConfig)

			if tc.dbFn != nil {
				tc.dbFn(c)
			}

			config, err := c.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, config, tc.wantConfig)
		})
	}
}
