package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingHandler_Simple(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
	}

	req := httptest.NewRequest("GET", "/billing/enabled", nil)
	w := httptest.NewRecorder()

	handler.GetBillingEnabled(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Billing status retrieved", response["message"])
	assert.True(t, response["status"].(bool))

	data := response["data"].(map[string]interface{})
	assert.True(t, data["enabled"].(bool))
}

func TestBillingHandler_GetPlans(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	req := httptest.NewRequest("GET", "/billing/plans", nil)
	w := httptest.NewRecorder()

	handler.GetPlans(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Plans retrieved successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_GetTaxIDTypes(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	req := httptest.NewRequest("GET", "/billing/tax_id_types", nil)
	w := httptest.NewRecorder()

	handler.GetTaxIDTypes(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Tax ID types retrieved successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_GetInvoices(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	req := httptest.NewRequest("GET", "/organisations/test-org/billing/invoices", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "test-org")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	w := httptest.NewRecorder()

	handler.GetInvoices(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Invoices retrieved successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_GetSubscription(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	req := httptest.NewRequest("GET", "/organisations/test-org/billing/subscription", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "test-org")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	w := httptest.NewRecorder()

	handler.GetSubscription(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Subscription retrieved successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_CreateOrganisation(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	orgData := map[string]interface{}{
		"name":          "Test Org",
		"billing_email": "test@example.com",
	}

	body, _ := json.Marshal(orgData)
	req := httptest.NewRequest("POST", "/organisations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateOrganisation(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Organisation created successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_GetOrganisation(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	req := httptest.NewRequest("GET", "/organisations/org-1", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	w := httptest.NewRecorder()

	handler.GetOrganisation(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Organisation retrieved successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_UpdateOrganisation(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	orgData := map[string]interface{}{
		"name": "Updated Org",
	}

	body, _ := json.Marshal(orgData)
	req := httptest.NewRequest("PUT", "/organisations/org-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	w := httptest.NewRecorder()

	handler.UpdateOrganisation(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Organisation updated successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_GetPaymentMethods(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	req := httptest.NewRequest("GET", "/organisations/test-org/billing/payment_methods", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "test-org")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	w := httptest.NewRecorder()

	handler.GetPaymentMethods(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Payment methods retrieved successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_CreateSubscription(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	subData := map[string]interface{}{
		"plan_id": "plan-1",
	}

	body, _ := json.Marshal(subData)
	req := httptest.NewRequest("POST", "/organisations/org-1/subscriptions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	w := httptest.NewRecorder()

	handler.CreateSubscription(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Subscription created successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_GetInvoice(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	req := httptest.NewRequest("GET", "/organisations/org-1/billing/invoices/inv-1", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "org-1")
	chiCtx.URLParams.Add("invoiceID", "inv-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	w := httptest.NewRecorder()

	handler.GetInvoice(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Invoice retrieved successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_GetSetupIntent(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	req := httptest.NewRequest("GET", "/organisations/org-1/payment_methods/setup_intent", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	w := httptest.NewRecorder()

	handler.GetSetupIntent(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Setup intent retrieved successfully", response["message"])
	assert.True(t, response["status"].(bool))
}


func TestBillingHandler_GetSubscriptions(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	req := httptest.NewRequest("GET", "/organisations/org-1/subscriptions", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	w := httptest.NewRecorder()

	handler.GetSubscriptions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Subscriptions retrieved successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_UpdateOrganisationTaxID(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	taxData := map[string]interface{}{
		"tax_id_type": "ein",
		"tax_number":  "12-3456789",
	}

	body, _ := json.Marshal(taxData)
	req := httptest.NewRequest("POST", "/organisations/org-1/tax_id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	w := httptest.NewRecorder()

	handler.UpdateOrganisationTaxID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Tax ID updated successfully", response["message"])
	assert.True(t, response["status"].(bool))
}

func TestBillingHandler_UpdateOrganisationAddress(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	apiOptions := &types.APIOptions{
		Cfg: config.Configuration{
			Billing: cfg,
		},
	}

	handler := &BillingHandler{
		Handler: &Handler{
			A: apiOptions,
		},
		BillingClient: &billing.MockBillingClient{},
	}

	addressData := map[string]interface{}{
		"billing_address": "123 Main St",
		"billing_city":    "New York",
		"billing_state":   "NY",
		"billing_zip":     "10001",
		"billing_country": "US",
	}

	body, _ := json.Marshal(addressData)
	req := httptest.NewRequest("POST", "/organisations/org-1/address", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	w := httptest.NewRecorder()

	handler.UpdateOrganisationAddress(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Address updated successfully", response["message"])
	assert.True(t, response["status"].(bool))
}
