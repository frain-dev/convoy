package models

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"gopkg.in/guregu/null.v4"
)

type Configuration struct {
	IsAnalyticsEnabled *bool                       `json:"is_analytics_enabled"`
	IsSignupEnabled    *bool                       `json:"is_signup_enabled"`
	StoragePolicy      *StoragePolicyConfiguration `json:"storage_policy"`
}

func (c *Configuration) Validate() error {
	return util.Validate(c)
}

type ConfigurationResponse struct {
	*datastore.Configuration
	ApiVersion string `json:"api_version"`
}

type StoragePolicyConfiguration struct {
	Type   datastore.StorageType `json:"type,omitempty" valid:"supported_storage~please provide a valid storage type,required"`
	S3     *S3Storage            `json:"s3"`
	OnPrem *OnPremStorage        `json:"on_prem"`
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
	Prefix       null.String `json:"prefix"`
	Bucket       null.String `json:"bucket" valid:"required~please provide a bucket name"`
	AccessKey    null.String `json:"access_key,omitempty" valid:"required~please provide an access key"`
	SecretKey    null.String `json:"secret_key,omitempty" valid:"required~please provide a secret key"`
	Region       null.String `json:"region,omitempty"`
	SessionToken null.String `json:"session_token"`
	Endpoint     null.String `json:"endpoint,omitempty"`
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
