package handlers

import (
	"errors"
	"strings"

	"github.com/frain-dev/convoy/internal/pkg/billing"
)

var ErrHostRequiredForBilling = errors.New("organisation host (assigned domain) is required for billing. Please set the assigned domain in the configuration")
var ErrOwnerEmailRequiredForBilling = errors.New("organisation owner email is required for billing")

type BillingHandler struct {
	*Handler
	BillingClient billing.Client
}

// headerOrganisationID carries the active organisation id on dashboard requests.
const headerOrganisationID = "X-Organisation-Id"

func isBillingOrgNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "organisation") && strings.Contains(s, "not found")
}
