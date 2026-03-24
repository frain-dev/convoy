package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type noCallBillingClient struct {
	*billing.MockBillingClient
}

func (noCallBillingClient) GetOrganisation(context.Context, string) (*billing.Response[billing.BillingOrganisation], error) {
	panic("billing client must not be called when billing is disabled")
}

func (noCallBillingClient) CreateOrganisation(context.Context, billing.BillingOrganisation) (*billing.Response[billing.BillingOrganisation], error) {
	panic("billing client must not be called when billing is disabled")
}

func TestGetInternalOrganisationID_BillingDisabled_DoesNotCallBilling(t *testing.T) {
	base := &billing.MockBillingClient{}
	client := noCallBillingClient{MockBillingClient: base}

	h := &BillingHandler{
		Handler: &Handler{
			A: &types.APIOptions{
				Cfg:           config.Configuration{Billing: config.BillingConfiguration{Enabled: false}},
				BillingClient: &client,
				Logger:        log.New("convoy", slog.LevelInfo),
			},
		},
		BillingClient: &client,
	}

	req := httptest.NewRequest(http.MethodGet, "/ui/billing/organisations/org-123/internal_id", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "org-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	w := httptest.NewRecorder()

	h.GetInternalOrganisationID(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Contains(t, body["message"], "Billing module is not enabled")
}

func TestIsBillingOrgNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "Organisation not found",
			err:  errors.New("Organisation not found"),
			want: true,
		},
		{
			name: "organisation not found lowercase",
			err:  errors.New("organisation not found"),
			want: true,
		},
		{
			name: "billing service error message",
			err:  errors.New("billing service error: Organisation not found"),
			want: true,
		},
		{
			name: "other error",
			err:  errors.New("failed to fetch organisation data"),
			want: false,
		},
		{
			name: "not found without organisation",
			err:  errors.New("resource not found"),
			want: false,
		},
		{
			name: "organisation without not found",
			err:  errors.New("organisation validation failed"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBillingOrgNotFound(tt.err)
			require.Equal(t, tt.want, got)
		})
	}
}
