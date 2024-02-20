package services

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/guregu/null/v5"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
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

func provideCreateConfigService(ctrl *gomock.Controller, newConfig *models.Configuration) *CreateConfigService {
	return &CreateConfigService{
		ConfigRepo: mocks.NewMockConfigurationRepository(ctrl),
		NewConfig:  newConfig,
	}
}

func TestCreateConfigService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx       context.Context
		newConfig *models.Configuration
	}

	tests := []struct {
		name       string
		args       args
		wantConfig *datastore.Configuration
		dbFn       func(c *CreateConfigService)
		wantErr    bool
		wantErrMsg string
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
			dbFn: func(c *CreateConfigService) {
				co, _ := c.ConfigRepo.(*mocks.MockConfigurationRepository)
				co.EXPECT().CreateConfiguration(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			c := provideCreateConfigService(ctrl, tc.args.newConfig)

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
			require.Equal(t, config.IsAnalyticsEnabled, tc.wantConfig.IsAnalyticsEnabled)
			require.Equal(t, config.IsSignupEnabled, tc.wantConfig.IsSignupEnabled)
		})
	}
}
