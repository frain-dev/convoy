package models

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"gopkg.in/guregu/null.v4"
)

type Configuration struct {
	// Determines whether your convoy instance sends us analytical data e.g event count
	IsAnalyticsEnabled *bool `json:"is_analytics_enabled"`

	// Allow or disallow user signups on your instance
	IsSignupEnabled *bool `json:"is_signup_enabled"`

	// Used to configure where events removed by retention policies are stored
	StoragePolicy *StoragePolicyConfiguration `json:"storage_policy"`
}

func (c *Configuration) Validate() error {
	return util.Validate(c)
}

type ConfigurationResponse struct {
	*datastore.Configuration
	ApiVersion string `json:"api_version"`
}

type StoragePolicyConfiguration struct {
	// Storage policy type e.g on_prem or s3
	Type datastore.StorageType `json:"type,omitempty" valid:"supported_storage~please provide a valid storage type,required"`

	// S3 Bucket creds
	S3 *S3Storage `json:"s3"`

	// On_Prem directory
	OnPrem *OnPremStorage `json:"on_prem"`
}

func (sc *StoragePolicyConfiguration) Transform() *datastore.StoragePolicyConfiguration {
	if sc == nil {
		return nil
	}

	return &datastore.StoragePolicyConfiguration{
		Type:   sc.Type,
		S3:     sc.S3.transform(),
		OnPrem: sc.OnPrem.transform(),
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
