package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideConfigService(ctrl *gomock.Controller) *ConfigService {
	configRepo := mocks.NewMockConfigurationRepository(ctrl)
	return NewConfigService(configRepo)
}

func TestConfigService_CreateOrUpdateConfiguration(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx       context.Context
		newConfig *models.Configuration
	}

	tests := []struct {
		name        string
		args        args
		wantConfig  *datastore.Configuration
		dbFn        func(c *ConfigService)
		wantErr     bool
		wantErrCode int
	}{
		{
			name: "should_create_configuration",
			args: args{
				ctx:       ctx,
				newConfig: &models.Configuration{IsAnalyticsEnabled: boolPtr(true)},
			},
			wantConfig: &datastore.Configuration{IsAnalyticsEnabled: true},
			dbFn: func(c *ConfigService) {
				co, _ := c.configRepo.(*mocks.MockConfigurationRepository)
				co.EXPECT().LoadConfiguration(gomock.Any()).Times(1).Return(nil, datastore.ErrConfigNotFound)
				co.EXPECT().CreateConfiguration(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},

		{
			name: "should_update_configuration",
			args: args{
				ctx:       ctx,
				newConfig: &models.Configuration{IsAnalyticsEnabled: boolPtr(false)},
			},
			wantConfig: &datastore.Configuration{IsAnalyticsEnabled: false},
			dbFn: func(c *ConfigService) {
				co, _ := c.configRepo.(*mocks.MockConfigurationRepository)
				co.EXPECT().LoadConfiguration(gomock.Any()).Times(1).Return(&datastore.Configuration{IsAnalyticsEnabled: true}, nil)
				co.EXPECT().UpdateConfiguration(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},

		{
			name: "should_fail_to_create_configuration",
			args: args{
				ctx:       ctx,
				newConfig: &models.Configuration{IsAnalyticsEnabled: boolPtr(true)},
			},
			dbFn: func(c *ConfigService) {
				co, _ := c.configRepo.(*mocks.MockConfigurationRepository)
				co.EXPECT().LoadConfiguration(gomock.Any()).Times(1).Return(nil, datastore.ErrConfigNotFound)
				co.EXPECT().CreateConfiguration(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			c := provideConfigService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(c)
			}

			config, err := c.CreateOrUpdateConfiguration(tc.args.ctx, tc.args.newConfig)

			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				return
			}

			require.Nil(t, err)
			require.Equal(t, config.IsAnalyticsEnabled, tc.wantConfig.IsAnalyticsEnabled)
		})
	}
}

func TestConfigService_LoadConfiguration(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx context.Context
	}

	tests := []struct {
		name        string
		args        args
		dbFn        func(c *ConfigService)
		wantConfig  *datastore.Configuration
		wantErr     bool
		wantErrCode int
	}{
		{
			name:       "should_load_configuration",
			args:       args{ctx: ctx},
			wantConfig: &datastore.Configuration{UID: "12345", IsAnalyticsEnabled: true},
			dbFn: func(c *ConfigService) {
				co, _ := c.configRepo.(*mocks.MockConfigurationRepository)
				co.EXPECT().LoadConfiguration(gomock.Any()).Times(1).Return(&datastore.Configuration{UID: "12345", IsAnalyticsEnabled: true}, nil)
			},
		},

		{
			name:       "should_fail_load_configuration",
			args:       args{ctx: ctx},
			wantConfig: &datastore.Configuration{UID: "12345", IsAnalyticsEnabled: true},
			dbFn: func(c *ConfigService) {
				co, _ := c.configRepo.(*mocks.MockConfigurationRepository)
				co.EXPECT().LoadConfiguration(gomock.Any()).Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			c := provideConfigService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(c)
			}

			config, err := c.LoadConfiguration(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantConfig, config)
		})
	}
}
