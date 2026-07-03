package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/util"
)

func billingClientErrorStatus(err error, defaultStatus int) int {
	var billErr *billing.Error
	if errors.As(err, &billErr) && billErr.StatusCode == http.StatusConflict {
		return http.StatusConflict
	}
	return defaultStatus
}

func billingClientErrorIsDefinitive(err error) bool {
	var billErr *billing.Error
	if !errors.As(err, &billErr) || billErr.StatusCode == 0 {
		return false
	}
	return billErr.StatusCode >= http.StatusBadRequest && billErr.StatusCode < http.StatusInternalServerError
}

func renderBillingClientError(w http.ResponseWriter, r *http.Request, err error, defaultStatus int) {
	status := billingClientErrorStatus(err, defaultStatus)
	_ = render.Render(w, r, util.NewErrorResponse(err.Error(), status))
}

func validatePlanAndHost(planID, host string) (string, error) {
	if planID == "" {
		return "", errors.New("plan_id is required and must be a valid UUID")
	}
	if host == "" {
		return "", errors.New("host is required")
	}
	return billing.CanonicalOrigin(host)
}

func validateResubscribeEmail(email, resubscribeKey string) error {
	if strings.TrimSpace(email) == "" && resubscribeKey == "" {
		return errors.New("email is required")
	}
	return nil
}

func optionalCanonicalHost(host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", nil
	}
	return billing.CanonicalOrigin(host)
}

func (h *BillingHandler) requireSelfHostedBillingRoute(w http.ResponseWriter, r *http.Request, unavailableMsg string) bool {
	if h.A.Cfg.UsesOrgBilling() {
		_ = render.Render(w, r, util.NewErrorResponse(unavailableMsg, http.StatusForbidden))
		return false
	}
	if !h.requireSelfHostedBillingAdmin(w, r) {
		return false
	}
	if h.BillingClient == nil {
		_ = render.Render(w, r, util.NewErrorResponse("billing service unavailable", http.StatusServiceUnavailable))
		return false
	}
	return true
}

func validatePlanAndHostRequired(planID, host string) error {
	if planID == "" {
		return errors.New("plan_id is required and must be a valid UUID")
	}
	if host == "" {
		return errors.New("host is required")
	}
	return nil
}

func decodeUpgradeSubscriptionRequest(w http.ResponseWriter, r *http.Request, canonicalizeHost bool) (billing.UpgradeSubscriptionRequest, bool) {
	var requestData billing.UpgradeSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return requestData, false
	}

	var host string
	var err error
	if canonicalizeHost {
		host, err = validatePlanAndHost(requestData.PlanID, requestData.Host)
	} else {
		err = validatePlanAndHostRequired(requestData.PlanID, requestData.Host)
		if err == nil {
			host = requestData.Host
		}
	}
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return requestData, false
	}
	requestData.Host = host
	return requestData, true
}
