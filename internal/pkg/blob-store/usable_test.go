package blobstore

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
)

func TestStoragePolicyUsable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  *datastore.StoragePolicyConfiguration
		wantErr string
	}{
		{
			name:    "nil policy",
			policy:  nil,
			wantErr: "storage policy is not configured",
		},
		{
			name:    "empty type",
			policy:  &datastore.StoragePolicyConfiguration{},
			wantErr: "storage policy type is empty",
		},
		{
			name: "on_prem empty path",
			policy: &datastore.StoragePolicyConfiguration{
				Type:   datastore.OnPrem,
				OnPrem: &datastore.OnPremStorage{Path: null.StringFrom("")},
			},
			wantErr: "on_prem storage path is empty",
		},
		{
			name: "on_prem /dev/null",
			policy: &datastore.StoragePolicyConfiguration{
				Type:   datastore.OnPrem,
				OnPrem: &datastore.OnPremStorage{Path: null.StringFrom("/dev/null")},
			},
			wantErr: "not usable for archive export",
		},
		{
			name: "on_prem valid tmp",
			policy: &datastore.StoragePolicyConfiguration{
				Type:   datastore.OnPrem,
				OnPrem: &datastore.OnPremStorage{Path: null.StringFrom("/tmp/convoy-archive")},
			},
		},
		{
			name: "s3 missing bucket",
			policy: &datastore.StoragePolicyConfiguration{
				Type: datastore.S3,
				S3: &datastore.S3Storage{
					AccessKey: null.StringFrom("ak"),
					SecretKey: null.StringFrom("sk"),
				},
			},
			wantErr: "s3 bucket is required",
		},
		{
			name: "s3 missing credentials",
			policy: &datastore.StoragePolicyConfiguration{
				Type: datastore.S3,
				S3: &datastore.S3Storage{
					Bucket: null.StringFrom("bucket"),
				},
			},
			wantErr: "s3 access_key and secret_key are required",
		},
		{
			name: "s3 access without secret",
			policy: &datastore.StoragePolicyConfiguration{
				Type: datastore.S3,
				S3: &datastore.S3Storage{
					Bucket:    null.StringFrom("bucket"),
					AccessKey: null.StringFrom("ak"),
				},
			},
			wantErr: "s3 access_key and secret_key are required",
		},
		{
			name: "s3 valid",
			policy: &datastore.StoragePolicyConfiguration{
				Type: datastore.S3,
				S3: &datastore.S3Storage{
					Bucket:    null.StringFrom("bucket"),
					AccessKey: null.StringFrom("ak"),
					SecretKey: null.StringFrom("sk"),
				},
			},
		},
		{
			name: "azure incomplete",
			policy: &datastore.StoragePolicyConfiguration{
				Type: datastore.AzureBlob,
				AzureBlob: &datastore.AzureBlobStorage{
					AccountName: null.StringFrom("acct"),
				},
			},
			wantErr: "azure account_key is required",
		},
		{
			name: "azure valid",
			policy: &datastore.StoragePolicyConfiguration{
				Type: datastore.AzureBlob,
				AzureBlob: &datastore.AzureBlobStorage{
					AccountName:   null.StringFrom("acct"),
					AccountKey:    null.StringFrom("key"),
					ContainerName: null.StringFrom("container"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := StoragePolicyUsable(tt.policy)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
