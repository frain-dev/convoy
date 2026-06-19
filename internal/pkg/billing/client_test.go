package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
)

func setupTestClient(t *testing.T) (*HTTPClient, *httptest.Server) {
	return setupTestClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(Response[map[string]interface{}]{
			Status:  true,
			Message: "Success",
			Data:    map[string]interface{}{"test": "data"},
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
}

func setupTestClientWithHandler(t *testing.T, handler http.Handler) (*HTTPClient, *httptest.Server) {
	server := httptest.NewServer(handler)
	cfg := config.BillingConfiguration{
		URL:    server.URL,
		APIKey: "test-key",
	}
	client := NewClient(cfg)
	return client, server
}

func setupTestClientWithResponse[T any](t *testing.T, data T) (*HTTPClient, *httptest.Server) {
	return setupTestClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(Response[T]{
			Status:  true,
			Message: "Success",
			Data:    data,
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
}

func TestNewClient(t *testing.T) {
	cfg := config.BillingConfiguration{
		URL:    "http://localhost:8080",
		APIKey: "test-key",
	}

	client := NewClient(cfg)

	assert.NotNil(t, client)
	assert.Equal(t, cfg, client.config)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
}

func TestClient_HealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/up", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))

	cfg := config.BillingConfiguration{
		URL:    server.URL,
		APIKey: "test-key",
	}

	client := NewClient(cfg)
	defer server.Close()

	err := client.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestClient_HealthCheck_NoURL(t *testing.T) {
	cfg := config.BillingConfiguration{
		URL:    "",
		APIKey: "test-key",
	}

	client := NewClient(cfg)

	err := client.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "billing service URL is not configured")
}

func TestClient_HealthCheck_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	cfg := config.BillingConfiguration{
		URL:    server.URL,
		APIKey: "test-key",
	}

	client := NewClient(cfg)
	defer server.Close()

	err := client.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "billing service health check failed")
}

func TestClient_GetUsage_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, Usage{})
	defer server.Close()

	resp, err := client.GetUsage(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetUsage_NoURL(t *testing.T) {
	cfg := config.BillingConfiguration{
		APIKey: "test-key",
	}

	client := NewClient(cfg)

	resp, err := client.GetUsage(context.Background(), "test-org")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "billing service URL is not configured")
}

func TestClient_GetInvoices_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, []Invoice{})
	defer server.Close()

	resp, err := client.GetInvoices(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetPaymentMethods_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, []PaymentMethod{})
	defer server.Close()

	resp, err := client.GetPaymentMethods(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetSubscription_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, BillingSubscription{})
	defer server.Close()

	resp, err := client.GetSubscription(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetPlans_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, []Plan{})
	defer server.Close()

	resp, err := client.GetPlans(context.Background())
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetTaxIDTypes_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/tax_id_types", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode([]TaxIDType{}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))

	cfg := config.BillingConfiguration{
		URL:    server.URL,
		APIKey: "test-key",
	}

	client := NewClient(cfg)
	defer server.Close()

	resp, err := client.GetTaxIDTypes(context.Background())
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Tax ID types retrieved successfully", resp.Message)
}

func TestClient_CreateOrganisation_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, BillingOrganisation{})
	defer server.Close()

	orgData := BillingOrganisation{
		Name:         "Test Org",
		BillingEmail: "test@example.com",
	}

	resp, err := client.CreateOrganisation(context.Background(), orgData)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetOrganisation_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, BillingOrganisation{})
	defer server.Close()

	resp, err := client.GetOrganisation(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_UpdateOrganisation_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, BillingOrganisation{})
	defer server.Close()

	orgData := BillingOrganisation{
		Name: "Updated Org",
	}

	resp, err := client.UpdateOrganisation(context.Background(), "test-org", orgData)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_UpdateOrganisationTaxID_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, BillingOrganisation{})
	defer server.Close()

	taxData := UpdateOrganisationTaxIDRequest{
		TaxIDType: "ein",
		TaxNumber: "12-3456789",
	}

	resp, err := client.UpdateOrganisationTaxID(context.Background(), "test-org", taxData)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_UpdateOrganisationAddress_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, BillingOrganisation{})
	defer server.Close()

	addressData := UpdateOrganisationAddressRequest{
		BillingName:    "Acme Billing",
		BillingAddress: "123 Main St",
		BillingCity:    "New York",
		BillingState:   "NY",
		BillingZip:     "10001",
		BillingCountry: "US",
	}

	resp, err := client.UpdateOrganisationAddress(context.Background(), "test-org", addressData)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetSubscriptions_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, []BillingSubscription{})
	defer server.Close()

	resp, err := client.GetSubscriptions(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_OnboardSubscription_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, Checkout{})
	defer server.Close()

	req := OnboardSubscriptionRequest{
		PlanID: "plan-uuid-123",
		Host:   "https://app.getconvoy.io",
	}

	resp, err := client.OnboardSubscription(context.Background(), "test-org", req)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_StartGuestCheckout_Success(t *testing.T) {
	client, server := setupTestClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/public/self_hosted_checkouts/start", r.URL.Path)
		assert.Empty(t, r.Header.Get("Authorization"))

		var req StartGuestCheckoutRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "buyer@example.com", req.Email)
		assert.Equal(t, "Acme", req.OrganisationName)
		assert.Equal(t, "attempt_123", req.AttemptID)
		assert.NotEmpty(t, req.CheckoutNonceHash)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(Response[Checkout]{
			Status:  true,
			Message: "Self-hosted checkout started",
			Data:    Checkout{CheckoutURL: "https://checkout.example", CheckoutID: "checkout_123", AttemptID: "attempt_123"},
		}))
	}))
	defer server.Close()

	resp, err := client.StartGuestCheckout(context.Background(), StartGuestCheckoutRequest{
		Email:             "buyer@example.com",
		PlanID:            "plan_123",
		Host:              "https://customer.example.com",
		OrganisationName:  "Acme",
		AttemptID:         "attempt_123",
		CheckoutNonceHash: "nonce_hash",
	})
	require.NoError(t, err)
	assert.Equal(t, "checkout_123", resp.Data.CheckoutID)
}

func TestClient_CompleteGuestCheckout_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, GuestCheckoutCompletion{
		Status:     "completed",
		LicenseKey: "license-key",
		CheckoutID: "checkout_123",
		ExternalID: "sh_ck_attempt_123",
	})
	defer server.Close()

	resp, err := client.CompleteGuestCheckout(context.Background(), CompleteGuestCheckoutRequest{
		Token:         "signed-token",
		AttemptID:     "attempt_123",
		CheckoutID:    "checkout_123",
		CheckoutNonce: "nonce",
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", resp.Data.Status)
	assert.Equal(t, "license-key", resp.Data.LicenseKey)
}

func TestClient_UpgradeSubscription_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, Checkout{})
	defer server.Close()

	req := UpgradeSubscriptionRequest{
		PlanID: "plan-uuid-456",
		Host:   "https://app.getconvoy.io",
	}

	resp, err := client.UpgradeSubscription(context.Background(), "test-org", "sub-123", req)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_DeleteSubscription_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, interface{}(nil))
	defer server.Close()

	resp, err := client.DeleteSubscription(context.Background(), "test-org", "test-subscription-id")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetSetupIntent_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, SetupIntent{})
	defer server.Close()

	resp, err := client.GetSetupIntent(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_DeletePaymentMethod_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, interface{}(nil))
	defer server.Close()

	resp, err := client.DeletePaymentMethod(context.Background(), "test-org", "pm-1")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetInvoice_Success(t *testing.T) {
	client, server := setupTestClientWithResponse(t, Invoice{})
	defer server.Close()

	resp, err := client.GetInvoice(context.Background(), "test-org", "inv-1")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestBillingClientPublicSelfHostedCallsDoNotSendBearerToken(t *testing.T) {
	seen := make(map[string]string)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path] = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "message": "ok", "data": []interface{}{}})
	}))
	defer server.Close()

	client := NewClient(config.BillingConfiguration{URL: server.URL})

	_, err := client.GetPlans(context.Background())
	require.NoError(t, err)

	require.Equal(t, "", seen["/public/self_hosted/plans"])
}

func TestBillingClientCloudPlanCatalogSendsBearerToken(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/plans", r.URL.Path)
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "message": "ok", "data": []interface{}{}})
	}))
	defer server.Close()

	client := NewClient(config.BillingConfiguration{URL: server.URL, APIKey: "cloud-api-key"})

	_, err := client.GetPlans(context.Background())
	require.NoError(t, err)

	require.Equal(t, "Bearer cloud-api-key", authHeader)
}

func TestBillingClientSelfHostedBillingUsesLicenseProof(t *testing.T) {
	var authHeader string
	var licenseHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/public/self_hosted_billing/subscription", r.URL.Path)
		authHeader = r.Header.Get("Authorization")
		licenseHeader = r.Header.Get("X-Convoy-License-Key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "message": "ok", "data": map[string]interface{}{"id": "sub_123"}})
	}))
	defer server.Close()

	client := NewClient(config.BillingConfiguration{URL: server.URL})

	_, err := client.GetSelfHostedSubscription(context.Background(), "license-key")
	require.NoError(t, err)

	require.Equal(t, "", authHeader)
	require.Equal(t, "license-key", licenseHeader)
}
