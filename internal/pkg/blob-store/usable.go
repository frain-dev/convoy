package blobstore

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/frain-dev/convoy/datastore"
)

// StoragePolicyUsable reports whether the storage policy can receive archive
// uploads. Call before claiming or scanning so unusable backends (for example
// on_prem path /dev/null) never run ExportRecords.
func StoragePolicyUsable(policy *datastore.StoragePolicyConfiguration) error {
	if policy == nil {
		return errors.New("storage policy is not configured")
	}

	switch policy.Type {
	case datastore.OnPrem:
		if policy.OnPrem == nil {
			return errors.New("on_prem storage path is not configured")
		}
		path := strings.TrimSpace(policy.OnPrem.Path.ValueOrZero())
		if path == "" {
			return errors.New("on_prem storage path is empty")
		}
		cleaned := filepath.Clean(path)
		if cleaned == "/dev/null" || cleaned == `\\.\NUL` {
			return fmt.Errorf("on_prem storage path %q is not usable for archive export", path)
		}
		return nil

	case datastore.S3:
		if policy.S3 == nil {
			return errors.New("s3 storage is not configured")
		}
		if strings.TrimSpace(policy.S3.Bucket.ValueOrZero()) == "" {
			return errors.New("s3 bucket is required for archive export")
		}
		access := strings.TrimSpace(policy.S3.AccessKey.ValueOrZero())
		secret := strings.TrimSpace(policy.S3.SecretKey.ValueOrZero())
		if access == "" && secret == "" {
			return errors.New("s3 access_key and secret_key are required for archive export")
		}
		return nil

	case datastore.AzureBlob:
		if policy.AzureBlob == nil {
			return errors.New("azure_blob storage is not configured")
		}
		if strings.TrimSpace(policy.AzureBlob.AccountName.ValueOrZero()) == "" {
			return errors.New("azure account_name is required for archive export")
		}
		if strings.TrimSpace(policy.AzureBlob.AccountKey.ValueOrZero()) == "" {
			return errors.New("azure account_key is required for archive export")
		}
		if strings.TrimSpace(policy.AzureBlob.ContainerName.ValueOrZero()) == "" {
			return errors.New("azure container_name is required for archive export")
		}
		return nil

	case "":
		return errors.New("storage policy type is empty")

	default:
		return fmt.Errorf("unknown storage policy type %q", policy.Type)
	}
}
