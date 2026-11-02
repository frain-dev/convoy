package models

import (
	"errors"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type Configuration struct {
	// Allow or disallow user signups on your instance
	IsSignupEnabled *bool `json:"is_signup_enabled"`

	// Selects Admin/DB values instead of environment values after restart.
	AdminManaged *bool `json:"admin_managed"`

	// Used to configure where events removed by retention policies are stored
	StoragePolicy *StoragePolicyConfiguration `json:"storage_policy"`

	// Keep window for partition drop (period only).
	RetentionPolicy *RetentionPolicyConfiguration `json:"retention_policy"`

	// Cold-storage archive/export enable.
	WebhookArchiving *WebhookArchivingConfiguration `json:"webhook_archiving"`
}

func (c *Configuration) Validate() error {
	if err := util.Validate(c); err != nil {
		return err
	}
	return c.validateRetentionPeriod()
}

// ValidateForUpdate is for PUT /ui/configuration. GetConfiguration redacts
// storage secrets and the Admin form resubmits the redacted shape, so blank
// access keys / on-prem paths mean "keep stored values" (see
// preserveStoragePolicySecrets), not "clear". Full Validate would reject those
// blanks via required tags meant for create.
//
// When storage_policy is present, type must be a concrete supported value.
// An empty type still replaces the stored policy in UpdateConfigService and
// clears every storage column on write.
func (c *Configuration) ValidateForUpdate() error {
	if c.StoragePolicy != nil {
		switch c.StoragePolicy.Type {
		case datastore.OnPrem, datastore.S3, datastore.AzureBlob:
			// ok
		default:
			return errors.New("please provide a valid storage type")
		}
	}
	return c.validateRetentionPeriod()
}

func (c *Configuration) validateRetentionPeriod() error {
	if c.RetentionPolicy == nil {
		return nil
	}
	period := c.RetentionPolicy.Period
	if util.IsStringEmpty(period) {
		period = c.RetentionPolicy.Policy
	}
	if util.IsStringEmpty(period) {
		return nil
	}
	if _, err := time.ParseDuration(period); err != nil {
		return errors.New("please provide a valid retention period duration")
	}
	return nil
}

type RetentionPolicyConfiguration struct {
	// Keep window for licensed partition drop (e.g. 720h).
	Period string `json:"period" valid:"duration~please provide a valid retention period duration"`

	// Deprecated: use Period.
	Policy string `json:"policy"`

	// Gates licensed 01:00 partition drop. Pointer so omitted JSON keeps the
	// stored value on update (period-only clients must not force-disable).
	Enabled *bool `json:"enabled"`
}

func (r *RetentionPolicyConfiguration) Transform() *datastore.RetentionPolicyConfiguration {
	if r == nil {
		return nil
	}

	period := r.Period
	if util.IsStringEmpty(period) {
		period = r.Policy
	}

	out := &datastore.RetentionPolicyConfiguration{
		Period:  period,
		Enabled: true,
	}
	if r.Enabled != nil {
		out.Enabled = *r.Enabled
	}
	return out
}

type WebhookArchivingConfiguration struct {
	Enabled bool `json:"enabled"`
}

func (w *WebhookArchivingConfiguration) Transform() *datastore.WebhookArchivingConfiguration {
	if w == nil {
		return nil
	}

	return &datastore.WebhookArchivingConfiguration{Enabled: w.Enabled}
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
