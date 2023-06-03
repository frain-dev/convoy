package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	return &s
}

func stripVariableFields(t *testing.T, obj string, v interface{}) {
	switch obj {
	case "project":
		g := v.(*datastore.Project)
		if g.Config != nil {
			for i := range g.Config.Signature.Versions {
				v := &g.Config.Signature.Versions[i]
				v.UID = ""
				v.CreatedAt = time.Time{}
			}
		}
		g.UID = ""
		g.CreatedAt, g.UpdatedAt, g.DeletedAt = time.Time{}, time.Time{}, null.Time{}
	case "endpoint":
		e := v.(*datastore.Endpoint)

		for i := range e.Secrets {
			s := &e.Secrets[i]
			s.UID = ""
			s.CreatedAt, s.UpdatedAt, s.DeletedAt = time.Time{}, time.Time{}, null.Time{}
		}

		e.UID, e.AppID = "", ""
		e.CreatedAt, e.UpdatedAt, e.DeletedAt = time.Time{}, time.Time{}, null.Time{}
	case "event":
		e := v.(*datastore.Event)
		e.UID = ""
		e.MatchedEndpoints = 0
		e.CreatedAt, e.UpdatedAt, e.DeletedAt = time.Time{}, time.Time{}, null.Time{}
	case "apiKey":
		a := v.(*datastore.APIKey)
		a.UID, a.MaskID, a.Salt, a.Hash = "", "", "", ""
		a.CreatedAt, a.UpdatedAt = time.Time{}, time.Time{}
	case "organisation":
		a := v.(*datastore.Organisation)
		a.UID = ""
		a.CreatedAt, a.UpdatedAt = time.Time{}, time.Time{}
	case "organisation_member":
		a := v.(*datastore.OrganisationMember)
		a.UID = ""
		a.CreatedAt, a.UpdatedAt = time.Time{}, time.Time{}
	case "organisation_invite":
		a := v.(*datastore.OrganisationInvite)
		a.UID = ""
		a.Token = ""
		a.CreatedAt, a.UpdatedAt, a.ExpiresAt, a.DeletedAt = time.Time{}, time.Time{}, time.Time{}, null.Time{}
	default:
		t.Errorf("invalid data body - %v of type %T", obj, obj)
		t.FailNow()
	}
}

func provideConfigService(ctrl *gomock.Controller) *ConfigService {
	configRepo := mocks.NewMockConfigurationRepository(ctrl)
	return NewConfigService(configRepo)
}

func TestConfigService_CreateConfiguration(t *testing.T) {
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
				ctx: ctx,
				newConfig: &models.Configuration{IsAnalyticsEnabled: boolPtr(true), IsSignupEnabled: boolPtr(true), StoragePolicy: &models.StoragePolicyConfiguration{
					Type: datastore.OnPrem,
					OnPrem: &models.OnPremStorage{
						Path: null.NewString("/tmp/", true),
					},
				}},
			},
			wantConfig: &datastore.Configuration{IsAnalyticsEnabled: true, IsSignupEnabled: true},
			dbFn: func(c *ConfigService) {
				co, _ := c.configRepo.(*mocks.MockConfigurationRepository)
				co.EXPECT().CreateConfiguration(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
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

			config, err := c.CreateConfiguration(tc.args.ctx, tc.args.newConfig)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				return
			}
			require.Nil(t, err)
			require.Equal(t, config.IsAnalyticsEnabled, tc.wantConfig.IsAnalyticsEnabled)
			require.Equal(t, config.IsSignupEnabled, tc.wantConfig.IsSignupEnabled)
		})
	}
}

func TestConfigService_UpdateConfiguration(t *testing.T) {
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
			dbFn: func(c *ConfigService) {
				co, _ := c.configRepo.(*mocks.MockConfigurationRepository)
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
			dbFn: func(c *ConfigService) {
				co, _ := c.configRepo.(*mocks.MockConfigurationRepository)
				co.EXPECT().LoadConfiguration(gomock.Any()).Times(1).Return(nil, datastore.ErrConfigNotFound)
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

			config, err := c.UpdateConfiguration(tc.args.ctx, tc.args.newConfig)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				return
			}

			require.Nil(t, err)
			require.Equal(t, config, tc.wantConfig)
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
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantConfig, config)
		})
	}
}
