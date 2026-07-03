package license

func IsProvisional(orgID, licenseData string) bool {
	if licenseData == "" {
		return false
	}
	payload, err := DecryptLicenseData(orgID, licenseData)
	return err == nil && payload != nil && payload.Provisional
}

func HasAuthoritativeEntitlements(orgID, licenseData string) bool {
	if licenseData == "" {
		return false
	}
	payload, err := DecryptLicenseData(orgID, licenseData)
	if err != nil || payload == nil || len(payload.Entitlements) == 0 {
		return false
	}
	return !payload.Provisional
}

func HasDailyEventLimitEntitlement(orgID, licenseData string) bool {
	if licenseData == "" {
		return false
	}
	payload, err := DecryptLicenseData(orgID, licenseData)
	if err != nil || payload == nil {
		return false
	}
	return EntitlementsHaveDailyEventLimit(payload.Entitlements)
}
