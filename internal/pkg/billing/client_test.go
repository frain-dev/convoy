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

func TestClient_GetUsage_NonJSONUpstreamReturnsBadGateway(t *testing.T) {
	client, server := setupTestClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<html>No route matches [GET] "/api/v1/organisations/org-1/usage"</html>`))
	}))
	defer server.Close()

	resp, err := client.GetUsage(context.Background(), "org-1")

	require.Nil(t, resp)
	var serviceErr *ServiceError
	require.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, http.StatusBadGateway, serviceErr.StatusCode)
	assert.Contains(t, serviceErr.Message, "billing returned non-JSON response")
	assert.Contains(t, serviceErr.Message, "HTTP 404")
}

func TestClient_GetUsage_AcceptsNullData(t *testing.T) {
	client, server := setupTestClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":true,"message":"ok","data":null}`))
	}))
	defer server.Close()

	resp, err := client.GetUsage(context.Background(), "org-1")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Status)
	assert.Equal(t, "ok", resp.Message)
}

func TestClient_LicenseBillingGetSubscription_UsesLicenseHeaderAndAcceptsNullData(t *testing.T) {
	client, server := setupTestClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "lk_test", r.Header.Get("X-License-Key"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":true,"message":"ok","data":null}`))
	}))
	defer server.Close()

	resp, err := client.LicenseBillingGetSubscription(context.Background(), "lk_test")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Status)
	assert.Equal(t, "ok", resp.Message)
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

func TestClient_GetSubscription_AcceptsArrayResponse(t *testing.T) {
	client, server := setupTestClientWithResponse(t, []BillingSubscription{{ID: "sub-1", Status: "active"}})
	defer server.Close()

	resp, err := client.GetSubscription(context.Background(), "test-org")
	require.NoError(t, err)
	assert.Equal(t, "sub-1", resp.Data.ID)
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

func TestClient_GetSubscriptions_AcceptsSingleObjectResponse(t *testing.T) {
	client, server := setupTestClientWithResponse(t, BillingSubscription{ID: "sub-1", Status: "active"})
	defer server.Close()

	resp, err := client.GetSubscriptions(context.Background(), "test-org")
	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "sub-1", resp.Data[0].ID)
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
