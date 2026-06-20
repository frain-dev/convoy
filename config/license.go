package config

import "strings"

// License key provenance, persisted on convoy.configurations.license_key_source.
const (
	LicenseSourceEnv           = "env"
	LicenseSourceGuestCheckout = "guest_checkout"
)

// ResolveEffectiveLicense applies env-first instance-license precedence: an
// explicit CONVOY_LICENSE_KEY / --license-key (envKey) wins, otherwise the
// persisted guest-checkout key (checkoutKey) is used. The returned source is the
// provenance to persist alongside the effective key. The checkout key is never
// mutated here; callers keep it in its own column so an env override stays
// reversible (removing env falls back to the purchased key on next boot).
func ResolveEffectiveLicense(envKey, checkoutKey string) (key, source string) {
	if e := strings.TrimSpace(envKey); e != "" {
		return e, LicenseSourceEnv
	}
	if c := strings.TrimSpace(checkoutKey); c != "" {
		return c, LicenseSourceGuestCheckout
	}
	return "", ""
}
