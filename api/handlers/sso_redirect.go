package handlers

import (
	"errors"
	"strings"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/billing"
)

var errSSORedirectHostNotApproved = errors.New("redirect URL host is not approved")

// validateSSOAdminPortalRedirectURL validates return_url/success_url before forwarding to Overwatch.
// Failure policy: fail closed. Cloud org-billing accepts only the instance host or request origin;
// licensed self-hosted accepts any canonical customer origin.
func validateSSOAdminPortalRedirectURL(raw string, cfg config.Configuration, requestOrigin string) (string, error) {
	canonical, err := billing.CanonicalOrigin(raw)
	if err != nil {
		return "", err
	}
	if !cfg.UsesOrgBilling() {
		return canonical, nil
	}

	if host := strings.TrimSpace(cfg.Host); host != "" {
		if allowed, err := billing.CanonicalOrigin(host); err == nil && canonical == allowed {
			return canonical, nil
		}
	}
	if requestOrigin != "" {
		if allowed, err := billing.CanonicalOrigin(requestOrigin); err == nil && canonical == allowed {
			return canonical, nil
		}
	}
	return "", errSSORedirectHostNotApproved
}
