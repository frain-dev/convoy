package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestClient(t *testing.T) (*HTTPClient, *httptest.Server) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(Response{
			Status:  true,
			Message: "Success",
			Data:    map[string]interface{}{"test": "data"},
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))

	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     server.URL,
		APIKey:  "test-key",
	}

	client := NewClient(cfg)
	return client, server
}

func TestNewClient(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
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
		Enabled: true,
		URL:     server.URL,
		APIKey:  "test-key",
	}

	client := NewClient(cfg)
	defer server.Close()

	err := client.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestClient_HealthCheck_Disabled(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: false,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	client := NewClient(cfg)

	err := client.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "billing is not enabled")
}

func TestClient_HealthCheck_NoURL(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "",
		APIKey:  "test-key",
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
		Enabled: true,
		URL:     server.URL,
		APIKey:  "test-key",
	}

	client := NewClient(cfg)
	defer server.Close()

	err := client.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "billing service health check failed")
}

func TestClient_GetUsage_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	resp, err := client.GetUsage(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetUsage_Disabled(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: false,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	client := NewClient(cfg)

	resp, err := client.GetUsage(context.Background(), "test-org")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "billing is not enabled")
}

func TestClient_GetInvoices_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	resp, err := client.GetInvoices(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetPaymentMethods_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	resp, err := client.GetPaymentMethods(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetSubscription_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	resp, err := client.GetSubscription(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetPlans_Success(t *testing.T) {
	client, server := setupTestClient(t)
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
		// Return raw array, not Response object
		if err := json.NewEncoder(w).Encode([]interface{}{"test", "data"}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))

	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     server.URL,
		APIKey:  "test-key",
	}

	client := NewClient(cfg)
	defer server.Close()

	resp, err := client.GetTaxIDTypes(context.Background())
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Tax ID types retrieved successfully", resp.Message)
}

func TestClient_CreateOrganisation_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	orgData := map[string]interface{}{
		"name":          "Test Org",
		"billing_email": "test@example.com",
	}

	resp, err := client.CreateOrganisation(context.Background(), orgData)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetOrganisation_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	resp, err := client.GetOrganisation(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_UpdateOrganisation_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	orgData := map[string]interface{}{
		"name": "Updated Org",
	}

	resp, err := client.UpdateOrganisation(context.Background(), "test-org", orgData)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_UpdateOrganisationTaxID_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	taxData := map[string]interface{}{
		"tax_id_type": "ein",
		"tax_number":  "12-3456789",
	}

	resp, err := client.UpdateOrganisationTaxID(context.Background(), "test-org", taxData)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_UpdateOrganisationAddress_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	addressData := map[string]interface{}{
		"billing_address": "123 Main St",
		"billing_city":    "New York",
		"billing_state":   "NY",
		"billing_zip":     "10001",
		"billing_country": "US",
	}

	resp, err := client.UpdateOrganisationAddress(context.Background(), "test-org", addressData)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetSubscriptions_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	resp, err := client.GetSubscriptions(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_CreateSubscription_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	subData := map[string]interface{}{
		"plan_id": "plan-1",
	}

	resp, err := client.CreateSubscription(context.Background(), "test-org", subData)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_UpdateSubscription_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	subData := map[string]interface{}{
		"plan_id": "plan-2",
	}

	resp, err := client.UpdateSubscription(context.Background(), "test-org", subData)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_DeleteSubscription_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	resp, err := client.DeleteSubscription(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetSetupIntent_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	resp, err := client.GetSetupIntent(context.Background(), "test-org")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_CreatePaymentMethod_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	pmData := map[string]interface{}{
		"payment_method_id": "pm_test_123",
	}

	resp, err := client.CreatePaymentMethod(context.Background(), "test-org", pmData)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_DeletePaymentMethod_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	resp, err := client.DeletePaymentMethod(context.Background(), "test-org", "pm-1")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_GetInvoice_Success(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	resp, err := client.GetInvoice(context.Background(), "test-org", "inv-1")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_makeRequest_Disabled(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: false,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	client := NewClient(cfg)

	resp, err := client.makeRequest(context.Background(), "GET", "/test", nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "billing is not enabled")
}

func TestClient_makeRequest_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(Response{
			Status:  false,
			Message: "Server error",
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))

	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     server.URL,
		APIKey:  "test-key",
	}

	client := NewClient(cfg)
	defer server.Close()

	resp, err := client.makeRequest(context.Background(), "GET", "/test", nil)
	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Status)
	assert.Equal(t, "Server error", resp.Message)
}

func TestClient_makeRequest_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("invalid json")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))

	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     server.URL,
		APIKey:  "test-key",
	}

	client := NewClient(cfg)
	defer server.Close()

	resp, err := client.makeRequest(context.Background(), "GET", "/test", nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to read billing response")
}

func TestClient_makeRequest_WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		assert.Equal(t, "test value", body["test"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(Response{
			Status:  true,
			Message: "Success",
			Data:    body,
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))

	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     server.URL,
		APIKey:  "test-key",
	}

	client := NewClient(cfg)
	defer server.Close()

	body := map[string]interface{}{
		"test": "test value",
	}

	resp, err := client.makeRequest(context.Background(), "POST", "/test", body)
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "Success", resp.Message)
}

func TestClient_makeRequest_InvalidBody(t *testing.T) {
	cfg := config.BillingConfiguration{
		Enabled: true,
		URL:     "http://localhost:8080",
		APIKey:  "test-key",
	}

	client := NewClient(cfg)

	invalidBody := make(chan int)
	resp, err := client.makeRequest(context.Background(), "POST", "/test", invalidBody)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to marshal request body")
}
