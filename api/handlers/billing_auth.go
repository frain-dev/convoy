package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/util"
)

func (h *BillingHandler) checkBillingAccess(w http.ResponseWriter, r *http.Request, orgID string) bool {
	if !h.A.Cfg.UsesOrgBilling() {
		_ = render.Render(w, r, util.NewErrorResponse("cloud org billing is not configured", http.StatusServiceUnavailable))
		return false
	}

	if h.BillingClient == nil {
		_ = render.Render(w, r, util.NewErrorResponse("billing service unavailable", http.StatusServiceUnavailable))
		return false
	}

	org, err := h.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("organisation not found", http.StatusNotFound))
		return false
	}

	if org.UID != orgID {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID mismatch", http.StatusForbidden))
		return false
	}

	if err := h.A.Authz.Authorize(r.Context(), string(policies.PermissionBillingManage), org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: billing access requires billing admin or organisation admin role", http.StatusForbidden))
		return false
	}

	return true
}

// orgGuard extracts the orgID path param and enforces billing access. It renders the
// appropriate error itself (400 when the id is missing, or the checkBillingAccess errors)
// and returns ok=false when the caller should stop.
func (h *BillingHandler) orgGuard(w http.ResponseWriter, r *http.Request) (string, bool) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return "", false
	}
	if !h.checkBillingAccess(w, r, orgID) {
		return "", false
	}
	return orgID, true
}

func (h *BillingHandler) getOwnerEmail(ctx context.Context, orgID string) string {
	orgRepo := h.orgRepo()
	org, err := orgRepo.FetchOrganisationByID(ctx, orgID)
	if err != nil {
		return ""
	}

	userRepo := users.New(h.A.Logger, h.A.DB)
	owner, err := userRepo.FindUserByID(ctx, org.OwnerID)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(owner.Email)
}

func (h *BillingHandler) requireSelfHostedBillingAdmin(w http.ResponseWriter, r *http.Request) bool {
	if h.canManageSelfHostedBilling(r) {
		return true
	}

	_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin, organisation admin, or billing admin access required to manage self-hosted billing", http.StatusForbidden))
	return false
}

// canManageSelfHostedBilling gates self-hosted billing mutations and usage.
// Instance admin always passes. Cloud org billing never uses this path for org
// admins (UsesOrgBilling). On self-hosted: single-org instances allow org admin
// or billing admin; multi-org instances require instance admin only.
// Failure policy: fail closed on count errors and on multi-org non-instance-admins.
func (h *BillingHandler) canManageSelfHostedBilling(r *http.Request) bool {
	user, err := h.retrieveUser(r)
	if err != nil {
		return false
	}

	memberRepo := h.orgMemberRepo()
	if _, err = memberRepo.FetchInstanceAdminByUserID(r.Context(), user.UID); err == nil {
		return true
	}
	if h.A.Cfg.UsesOrgBilling() {
		return false
	}

	orgCount, err := h.orgRepo().CountOrganisations(r.Context())
	if err != nil || orgCount != 1 {
		return false
	}

	// Resolve the sole live org from the instance, not the request path and not
	// FetchAnyOrganisationAdminByUserID (that helper can match soft-deleted orgs).
	// Billing routes like /ui/billing/config often have no orgID in the URL.
	orgs, _, err := h.orgRepo().LoadOrganisationsPaged(r.Context(), datastore.Pageable{
		PerPage:    1,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	})
	if err != nil || len(orgs) != 1 {
		return false
	}
	member, err := memberRepo.FetchOrganisationMemberByUserID(r.Context(), user.UID, orgs[0].UID)
	if err != nil || member == nil {
		return false
	}
	return member.Role.Type == auth.RoleOrganisationAdmin || member.Role.Type == auth.RoleBillingAdmin
}

func newCheckoutNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashCheckoutNonce(nonce string) string {
	sum := sha256.Sum256([]byte(nonce))
	return hex.EncodeToString(sum[:])
}

func (h *BillingHandler) activeOrganisationName(ctx context.Context, r *http.Request) string {
	orgID := strings.TrimSpace(r.Header.Get(headerOrganisationID))
	if orgID == "" {
		return ""
	}

	orgRepo := h.orgRepo()
	org, err := orgRepo.FetchOrganisationByID(ctx, orgID)
	if err != nil || org == nil {
		return ""
	}

	return strings.TrimSpace(org.Name)
}
