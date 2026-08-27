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

func TestUpdateConfigService_RetentionPartialPreservesPeriod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	off := false
	svc := provideUpdateConfigService(ctrl, &models.Configuration{
		RetentionPolicy: &models.RetentionPolicyConfiguration{Enabled: &off},
	})

	co := svc.ConfigRepo.(*mocks.MockConfigurationRepository)
	co.EXPECT().LoadConfiguration(gomock.Any()).Return(&datastore.Configuration{
		RetentionPolicy: &datastore.RetentionPolicyConfiguration{
			Period:  "720h",
			Enabled: true,
		},
	}, nil)
	co.EXPECT().UpdateConfiguration(gomock.Any(), gomock.AssignableToTypeOf(&datastore.Configuration{})).
		DoAndReturn(func(_ context.Context, cfg *datastore.Configuration) error {
			require.Equal(t, "720h", cfg.RetentionPolicy.Period)
			require.False(t, cfg.RetentionPolicy.Enabled)
			return nil
		})

	got, err := svc.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, "720h", got.RetentionPolicy.Period)
	require.False(t, got.RetentionPolicy.Enabled)
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
				Endpoint:     null.StringFrom("https://minio.example"),
				Prefix:       null.StringFrom("archives/"),
			},
		}
		next := &datastore.StoragePolicyConfiguration{
			Type: datastore.S3,
			S3:   &datastore.S3Storage{Bucket: null.StringFrom("bucket")},
		}

		preserveStoragePolicySecrets(next, prev)

		require.Equal(t, "stored-access", next.S3.AccessKey.String)
		require.Equal(t, "stored-secret", next.S3.SecretKey.String)
		require.Equal(t, "stored-session", next.S3.SessionToken.String)
		require.Equal(t, "https://minio.example", next.S3.Endpoint.String)
		require.Equal(t, "archives/", next.S3.Prefix.String)
	})

	t.Run("nil azure subtree is restored when type is unchanged", func(t *testing.T) {
		prev := &datastore.StoragePolicyConfiguration{
			Type: datastore.AzureBlob,
			AzureBlob: &datastore.AzureBlobStorage{
				AccountName:   null.StringFrom("acct"),
				AccountKey:    null.StringFrom("stored-azure"),
				ContainerName: null.StringFrom("container"),
			},
		}
		next := &datastore.StoragePolicyConfiguration{Type: datastore.AzureBlob}

		preserveStoragePolicySecrets(next, prev)

		require.NotNil(t, next.AzureBlob)
		require.Equal(t, "stored-azure", next.AzureBlob.AccountKey.String)
		require.Equal(t, "acct", next.AzureBlob.AccountName.String)
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

func TestUpdateConfigService_AzureTypeWithoutNestedKeepsPrevious(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := provideUpdateConfigService(ctrl, &models.Configuration{
		IsAnalyticsEnabled: boolPtr(false),
		StoragePolicy: &models.StoragePolicyConfiguration{
			Type: datastore.AzureBlob,
			// Admin form has no azure fields; Transform leaves AzureBlob nil.
		},
	})
	co := svc.ConfigRepo.(*mocks.MockConfigurationRepository)
	co.EXPECT().LoadConfiguration(gomock.Any()).Return(&datastore.Configuration{
		IsAnalyticsEnabled: true,
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type: datastore.AzureBlob,
			AzureBlob: &datastore.AzureBlobStorage{
				AccountName:   null.StringFrom("acct"),
				AccountKey:    null.StringFrom("key"),
				ContainerName: null.StringFrom("c"),
			},
		},
	}, nil)
	co.EXPECT().UpdateConfiguration(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, cfg *datastore.Configuration) error {
			require.False(t, cfg.IsAnalyticsEnabled)
			require.Equal(t, datastore.AzureBlob, cfg.StoragePolicy.Type)
			require.Equal(t, "key", cfg.StoragePolicy.AzureBlob.AccountKey.String)
			return nil
		},
	)

	_, err := svc.Run(context.Background())
	require.NoError(t, err)
}

func TestUpdateConfigService_RejectsTypeSwitchWithoutTargetCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := provideUpdateConfigService(ctrl, &models.Configuration{
		StoragePolicy: &models.StoragePolicyConfiguration{
			Type: datastore.S3,
			S3: &models.S3Storage{
				Bucket: null.StringFrom("new-bucket"),
				// blank secrets; previous was on_prem so preserve cannot help
			},
		},
	})
	co := svc.ConfigRepo.(*mocks.MockConfigurationRepository)
	co.EXPECT().LoadConfiguration(gomock.Any()).Return(&datastore.Configuration{
		StoragePolicy: &datastore.StoragePolicyConfiguration{
			Type:   datastore.OnPrem,
			OnPrem: &datastore.OnPremStorage{Path: null.StringFrom("/old")},
		},
	}, nil)

	_, err := svc.Run(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "access_key and secret_key")
}

func TestAssertStoragePolicyFields(t *testing.T) {
	require.NoError(t, assertStoragePolicyFields(nil))
	require.NoError(t, assertStoragePolicyFields(&datastore.StoragePolicyConfiguration{
		Type:   datastore.OnPrem,
		OnPrem: &datastore.OnPremStorage{Path: null.StringFrom("/dev/null")},
	}))
	require.Error(t, assertStoragePolicyFields(&datastore.StoragePolicyConfiguration{
		Type: datastore.S3,
		S3:   &datastore.S3Storage{Bucket: null.StringFrom("b"), AccessKey: null.StringFrom("a")},
	}))
}
