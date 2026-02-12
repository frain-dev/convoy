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
