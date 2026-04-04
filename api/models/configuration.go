package models

import (
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type Configuration struct {
	// Determines whether your convoy instance sends us analytical data e.g event count
	IsAnalyticsEnabled *bool `json:"is_analytics_enabled"`

	// Allow or disallow user signups on your instance
	IsSignupEnabled *bool `json:"is_signup_enabled"`

	// Used to configure where events removed by retention policies are stored
	StoragePolicy *StoragePolicyConfiguration `json:"storage_policy"`

	// Used to configure whether the retention policy job runs and at what intervals
	RetentionPolicy *RetentionPolicyConfiguration
}

func (c *Configuration) Validate() error {
	return util.Validate(c)
}

type RetentionPolicyConfiguration struct {
	// Controls whether the retention policy is active on this instance.
	IsRetentionPolicyEnabled bool `json:"retention_policy_enabled"`

	// Specify the number of hours the policy job should go back before deleting events and deliveries.
	Policy string `json:"policy" valid:"duration~please provide a valid retention policy time duration"`
}

func (r *RetentionPolicyConfiguration) Transform() *datastore.RetentionPolicyConfiguration {
	if r == nil {
		return nil
	}

	return &datastore.RetentionPolicyConfiguration{Policy: r.Policy, IsRetentionPolicyEnabled: r.IsRetentionPolicyEnabled}
}

type ConfigurationResponse struct {
	*datastore.Configuration
	ApiVersion string `json:"api_version"`
}

type StoragePolicyConfiguration struct {
	// Storage policy type e.g on_prem, s3, or azure_blob
	Type datastore.StorageType `json:"type,omitempty" valid:"supported_storage~please provide a valid storage type,required"`

	// S3 Bucket creds
	S3 *S3Storage `json:"s3"`

	// On_Prem directory
	OnPrem *OnPremStorage `json:"on_prem"`

	// Azure Blob Storage creds
	AzureBlob *AzureBlobStorage `json:"azure_blob"`
}

func (sc *StoragePolicyConfiguration) Transform() *datastore.StoragePolicyConfiguration {
	if sc == nil {
		return nil
	}

	return &datastore.StoragePolicyConfiguration{
		Type:      sc.Type,
		S3:        sc.S3.transform(),
		OnPrem:    sc.OnPrem.transform(),
		AzureBlob: sc.AzureBlob.transform(),
	}
}

type S3Storage struct {
	// AWS  S3 Bucket Prefix
	Prefix null.String `json:"prefix"`

	// AWS S3 Bucket Name
	Bucket null.String `json:"bucket" valid:"required~please provide a bucket name"`

	// AWS Access Key
	AccessKey null.String `json:"access_key,omitempty" valid:"required~please provide an access key"`

	// AWS Secret Key
	SecretKey null.String `json:"secret_key,omitempty" valid:"required~please provide a secret key"`

	// AWS S3 Bucket Region
	Region null.String `json:"region,omitempty"`

	// AWS SessionToken
	SessionToken null.String `json:"session_token"`

	// AWS S3 Bucket SessionToken
	Endpoint null.String `json:"endpoint,omitempty"`
}

func (s3 *S3Storage) transform() *datastore.S3Storage {
	if s3 == nil {
		return nil
	}

	return &datastore.S3Storage{
		Prefix:       s3.Prefix,
		Bucket:       s3.Bucket,
		AccessKey:    s3.AccessKey,
		SecretKey:    s3.SecretKey,
		Region:       s3.Region,
		SessionToken: s3.SessionToken,
		Endpoint:     s3.Endpoint,
	}
}

type OnPremStorage struct {
	Path null.String `json:"path" db:"path"`
}

func (os *OnPremStorage) transform() *datastore.OnPremStorage {
	if os == nil {
		return nil
	}

	return &datastore.OnPremStorage{Path: os.Path}
}

type AzureBlobStorage struct {
	AccountName   null.String `json:"account_name"`
	AccountKey    null.String `json:"account_key,omitempty"`
	ContainerName null.String `json:"container_name"`
	Endpoint      null.String `json:"endpoint,omitempty"`
	Prefix        null.String `json:"prefix,omitempty"`
}

func (az *AzureBlobStorage) transform() *datastore.AzureBlobStorage {
	if az == nil {
		return nil
	}

	return &datastore.AzureBlobStorage{
		AccountName:   az.AccountName,
		AccountKey:    az.AccountKey,
		ContainerName: az.ContainerName,
		Endpoint:      az.Endpoint,
		Prefix:        az.Prefix,
	}
}
