package license

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFeatureListFromEntitlements_Empty(t *testing.T) {
	out, err := FeatureListFromEntitlements(nil)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &m))
	require.Empty(t, m)

	out, err = FeatureListFromEntitlements(map[string]interface{}{})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(out, &m))
	require.Empty(t, m)
}

func TestFeatureListFromEntitlements_LimitsAndBooleans(t *testing.T) {
	entitlements := map[string]interface{}{
		"org_limit":               int64(2),
		"user_limit":              int64(10),
		"project_limit":           int64(5),
		"enterprise_sso":          true,
		"portal_links":            true,
		"webhook_transformations": false,
	}
	out, err := FeatureListFromEntitlements(entitlements)
	require.NoError(t, err)

	var data map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &data))

	// limits
	for _, key := range []string{"org_limit", "user_limit", "project_limit"} {
		lim, ok := data[key].(map[string]interface{})
		require.True(t, ok, "missing %s", key)
		require.True(t, lim["allowed"].(bool))
		require.True(t, lim["available"].(bool))
		require.False(t, lim["limit_reached"].(bool))
		require.Equal(t, float64(0), lim["current"])
	}
	require.Equal(t, float64(2), data["org_limit"].(map[string]interface{})["limit"])
	require.Equal(t, float64(10), data["user_limit"].(map[string]interface{})["limit"])
	require.Equal(t, float64(5), data["project_limit"].(map[string]interface{})["limit"])

	// booleans
	require.True(t, data["EnterpriseSSO"].(bool))
	require.True(t, data["PortalLinks"].(bool))
	require.False(t, data["Transformations"].(bool))
}

func TestFeatureListFromEntitlementsWithUsage_Limits(t *testing.T) {
	entitlements := map[string]interface{}{
		"org_limit":     int64(1),
		"user_limit":    int64(1),
		"project_limit": int64(1),
	}

	tests := []struct {
		name             string
		key              string
		orgCount         int64
		memberCount      int64
		projectCount     int64
		wantAllowed      bool
		wantLimitReached bool
		wantCurrent      float64
	}{
		{"org at cap gates", "org_limit", 1, -1, -1, false, true, 1},
		{"org under cap allowed", "org_limit", 0, -1, -1, true, false, 0},
		{"org unknown fail-open", "org_limit", -1, -1, -1, true, false, 0},
		{"member at cap gates", "user_limit", -1, 1, -1, false, true, 1},
		{"member under cap allowed", "user_limit", -1, 0, -1, true, false, 0},
		{"member unknown fail-open", "user_limit", -1, -1, -1, true, false, 0},
		{"project at cap gates", "project_limit", -1, -1, 1, false, true, 1},
		{"project under cap allowed", "project_limit", -1, -1, 0, true, false, 0},
		{"project unknown fail-open", "project_limit", -1, -1, -1, true, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := FeatureListFromEntitlementsWithUsage(entitlements, tt.orgCount, tt.memberCount, tt.projectCount)
			require.NoError(t, err)

			var data map[string]interface{}
			require.NoError(t, json.Unmarshal(out, &data))

			block, ok := data[tt.key].(map[string]interface{})
			require.True(t, ok, "missing %s", tt.key)
			require.Equal(t, tt.wantAllowed, block["allowed"].(bool))
			require.Equal(t, tt.wantLimitReached, block["limit_reached"].(bool))
			require.Equal(t, tt.wantCurrent, block["current"])
		})
	}
}

func TestOrgEntitlementCap(t *testing.T) {
	orgID := "org-cap-1"

	tests := []struct {
		name        string
		licenseData string
		key         string
		wantLimit   int64
		wantApplies bool
	}{
		{
			name:        "empty license data fails open",
			licenseData: "",
			key:         "user_limit",
			wantApplies: false,
		},
		{
			name:        "wrong org key cannot decrypt, fails open",
			licenseData: encryptCap(t, "other-org", map[string]interface{}{"user_limit": int64(1)}),
			key:         "user_limit",
			wantApplies: false,
		},
		{
			name:        "absent entitlement fails open",
			licenseData: encryptCap(t, orgID, map[string]interface{}{"project_limit": int64(1)}),
			key:         "user_limit",
			wantApplies: false,
		},
		{
			name:        "unlimited (-1) fails open",
			licenseData: encryptCap(t, orgID, map[string]interface{}{"user_limit": int64(-1)}),
			key:         "user_limit",
			wantApplies: false,
		},
		{
			name:        "non-positive (0) fails open",
			licenseData: encryptCap(t, orgID, map[string]interface{}{"user_limit": int64(0)}),
			key:         "user_limit",
			wantApplies: false,
		},
		{
			name:        "finite cap applies",
			licenseData: encryptCap(t, orgID, map[string]interface{}{"user_limit": int64(1)}),
			key:         "user_limit",
			wantLimit:   1,
			wantApplies: true,
		},
		{
			name:        "org_limit finite cap applies",
			licenseData: encryptCap(t, orgID, map[string]interface{}{"org_limit": int64(2)}),
			key:         "org_limit",
			wantLimit:   2,
			wantApplies: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, applies := OrgEntitlementCap(orgID, tt.licenseData, tt.key)
			require.Equal(t, tt.wantApplies, applies)
			if tt.wantApplies {
				require.Equal(t, tt.wantLimit, limit)
			}
		})
	}
}

func encryptCap(t *testing.T, orgID string, entitlements map[string]interface{}) string {
	t.Helper()
	enc, err := EncryptLicenseData(orgID, &LicenseDataPayload{Key: "lk", Entitlements: entitlements})
	require.NoError(t, err)
	return enc
}

func TestFeatureListFromEntitlements_RoundtripWithCipher(t *testing.T) {
	orgID := "test-org"
	entitlements := map[string]interface{}{
		"enterprise_sso": true,
		"user_limit":     int64(25),
	}
	payload := &LicenseDataPayload{Key: "key", Entitlements: entitlements}
	enc, err := EncryptLicenseData(orgID, payload)
	require.NoError(t, err)

	dec, err := DecryptLicenseData(orgID, enc)
	require.NoError(t, err)
	require.NotNil(t, dec)
	require.Equal(t, "key", dec.Key)
	require.Equal(t, float64(25), dec.Entitlements["user_limit"])
	require.Equal(t, true, dec.Entitlements["enterprise_sso"])

	out, err := FeatureListFromEntitlements(dec.Entitlements)
	require.NoError(t, err)
	var data map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &data))
	require.True(t, data["EnterpriseSSO"].(bool))
	ul := data["user_limit"].(map[string]interface{})
	require.Equal(t, float64(25), ul["limit"])
}
