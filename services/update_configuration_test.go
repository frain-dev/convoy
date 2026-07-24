package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func provideUpdateConfigService(ctrl *gomock.Controller, config *models.Configuration) *UpdateConfigService {
	return &UpdateConfigService{
		ConfigRepo: mocks.NewMockConfigurationRepository(ctrl),
		Config:     config,
		Logger:     mocks.NewMockLogger(ctrl),
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

				ml, _ := c.Logger.(*mocks.MockLogger)
				ml.EXPECT().ErrorContext(gomock.Any(), "failed to load configuration", "error", gomock.Any()).Times(1)
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

func TestPreserveStoragePolicySecrets(t *testing.T) {
	t.Run("blank incoming secrets are preserved from previous within the same type", func(t *testing.T) {
		prev := &datastore.StoragePolicyConfiguration{
			Type: datastore.S3,
			S3: &datastore.S3Storage{
				Bucket:       null.StringFrom("bucket"),
				AccessKey:    null.StringFrom("stored-access"),
				SecretKey:    null.StringFrom("stored-secret"),
				SessionToken: null.StringFrom("stored-session"),
			},
			AzureBlob: &datastore.AzureBlobStorage{AccountKey: null.StringFrom("stored-azure")},
			OnPrem:    &datastore.OnPremStorage{Path: null.StringFrom("/stored/path")},
		}
		next := &datastore.StoragePolicyConfiguration{
			Type:      datastore.S3,
			S3:        &datastore.S3Storage{Bucket: null.StringFrom("bucket")},
			AzureBlob: &datastore.AzureBlobStorage{},
			OnPrem:    &datastore.OnPremStorage{},
		}

		preserveStoragePolicySecrets(next, prev)

		require.Equal(t, "stored-access", next.S3.AccessKey.String)
		require.Equal(t, "stored-secret", next.S3.SecretKey.String)
		require.Equal(t, "stored-session", next.S3.SessionToken.String)
		require.Equal(t, "stored-azure", next.AzureBlob.AccountKey.String)
		require.Equal(t, "/stored/path", next.OnPrem.Path.String)
	})

	t.Run("provided incoming secrets override previous", func(t *testing.T) {
		prev := &datastore.StoragePolicyConfiguration{
			Type: datastore.S3,
			S3:   &datastore.S3Storage{AccessKey: null.StringFrom("stored-access"), SecretKey: null.StringFrom("stored-secret")},
		}
		next := &datastore.StoragePolicyConfiguration{
			Type: datastore.S3,
			S3:   &datastore.S3Storage{AccessKey: null.StringFrom("new-access"), SecretKey: null.StringFrom("new-secret")},
		}

		preserveStoragePolicySecrets(next, prev)

		require.Equal(t, "new-access", next.S3.AccessKey.String)
		require.Equal(t, "new-secret", next.S3.SecretKey.String)
	})

	t.Run("switching storage type does not carry secrets over", func(t *testing.T) {
		prev := &datastore.StoragePolicyConfiguration{
			Type: datastore.S3,
			S3:   &datastore.S3Storage{AccessKey: null.StringFrom("stored-access")},
		}
		next := &datastore.StoragePolicyConfiguration{
			Type:   datastore.OnPrem,
			OnPrem: &datastore.OnPremStorage{Path: null.StringFrom("/new/path")},
		}

		preserveStoragePolicySecrets(next, prev)

		require.Nil(t, next.S3)
		require.Equal(t, "/new/path", next.OnPrem.Path.String)
	})

	t.Run("nil policies are a no-op", func(t *testing.T) {
		require.NotPanics(t, func() { preserveStoragePolicySecrets(nil, nil) })
		require.NotPanics(t, func() {
			preserveStoragePolicySecrets(&datastore.StoragePolicyConfiguration{}, nil)
		})
	})
}
