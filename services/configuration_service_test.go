package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideConfigService(ctrl *gomock.Controller) *ConfigService {
	configRepo := mocks.NewMockConfigurationRepository(ctrl)
	return NewConfigService(configRepo)
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
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantConfig, config)
		})
	}
}
