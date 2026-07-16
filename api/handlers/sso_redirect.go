package handlers

import (
	"errors"
	"strings"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/billing"
)

var errSSORedirectHostNotApproved = errors.New("redirect URL host is not approved")

// validateSSOAdminPortalRedirectURL validates return_url/success_url before forwarding to Overwatch.
// Failure policy: fail closed. Cloud org-billing accepts only the configured instance host
// (cfg.Host), with the same https:// defaulting used for bare host values elsewhere.
// Licensed self-hosted accepts any canonical customer origin.
func validateSSOAdminPortalRedirectURL(raw string, cfg config.Configuration) (string, error) {
	canonical, err := billing.CanonicalOrigin(raw)
	if err != nil {
		return "", err
	}
	if !cfg.UsesOrgBilling() {
		return canonical, nil
	}

	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return "", errSSORedirectHostNotApproved
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	allowed, err := billing.CanonicalOrigin(host)
	if err != nil || canonical != allowed {
		return "", errSSORedirectHostNotApproved
	}
	return canonical, nil
}
