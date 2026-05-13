package config

import "strings"

type BillingMode string

const (
	BillingModeCloud      BillingMode = "cloud"
	BillingModeLicensed   BillingMode = "licensed"
	BillingModeUnlicensed BillingMode = "unlicensed"
)

func (c *Configuration) Mode() BillingMode {
	if strings.TrimSpace(c.Billing.APIKey) != "" {
		return BillingModeCloud
	}
	if strings.TrimSpace(c.LicenseKey) != "" {
		return BillingModeLicensed
	}
	return BillingModeUnlicensed
}

func (c *Configuration) IsCloud() bool {
	return c.Mode() == BillingModeCloud
}

func (c *Configuration) IsSelfHosted() bool {
	return c.Mode() != BillingModeCloud
}
