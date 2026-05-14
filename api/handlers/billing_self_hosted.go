package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/util"
)

// SelfHostedRegisterEmail kicks off the buy flow for a self-hosted instance:
// the dashboard collects an email, Overwatch sends a verification code, and the
// next call (SelfHostedVerifyEmail) returns a license key for the user to set
// as CONVOY_LICENSE_KEY. Available on any self-hosted instance (licensed or
// unlicensed) so existing licensed users can re-issue if needed.
func (h *BillingHandler) SelfHostedRegisterEmail(w http.ResponseWriter, r *http.Request) {
	if !h.A.Cfg.IsSelfHosted() {
		_ = render.Render(w, r, util.NewErrorResponse("Self-hosted billing bootstrap is not available on managed cloud", http.StatusForbidden))
		return
	}
	if h.A.Authz != nil && !h.checkBillingCreateAccess(w, r) {
		return
	}

	var body struct {
		Email            string `json:"email"`
		OrganisationName string `json:"organisation_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}
	email := strings.TrimSpace(body.Email)
	if email == "" {
		_ = render.Render(w, r, util.NewErrorResponse("email is required", http.StatusBadRequest))
		return
	}

	resp, err := h.A.Billing.SelfHostedRegisterEmail(r.Context(), billing.SelfHostedRegisterEmailRequest{
		Email:            email,
		OrganisationName: strings.TrimSpace(body.OrganisationName),
	})
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(resp.Message, resp.Data, http.StatusOK))
}

// SelfHostedVerifyEmail accepts the verification code, returns the issued
// license key, and instructs the operator to set it as CONVOY_LICENSE_KEY and
// restart Convoy. Post-restart, the licenser switches from unlicensed to
// licensed mode automatically.
func (h *BillingHandler) SelfHostedVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if !h.A.Cfg.IsSelfHosted() {
		_ = render.Render(w, r, util.NewErrorResponse("Self-hosted billing bootstrap is not available on managed cloud", http.StatusForbidden))
		return
	}
	if h.A.Authz != nil && !h.checkBillingCreateAccess(w, r) {
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}
	if strings.TrimSpace(body.Code) == "" {
		_ = render.Render(w, r, util.NewErrorResponse("code is required", http.StatusBadRequest))
		return
	}
	code := strings.TrimSpace(body.Code)

	resp, err := h.A.Billing.SelfHostedVerifyEmail(r.Context(), code)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
		return
	}

	data := map[string]string{
		"external_id":        resp.Data.ExternalID,
		"masked_license_key": billing.MaskLicenseKey(resp.Data.LicenseKey),
		"instructions":       resp.Data.Instructions,
	}
	if strings.TrimSpace(data["instructions"]) == "" {
		data["instructions"] = "License issued and emailed. Set the license key as CONVOY_LICENSE_KEY, restart Convoy, then refresh this page."
	}
	_ = render.Render(w, r, util.NewServerResponse(resp.Message, data, http.StatusOK))
}
