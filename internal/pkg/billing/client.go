package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy/config"
)

type Client interface {
	HealthCheck(ctx context.Context) error
	GetUsage(ctx context.Context, orgID string) (*Response, error)
	GetInvoices(ctx context.Context, orgID string) (*Response, error)
	GetPaymentMethods(ctx context.Context, orgID string) (*Response, error)
	GetSubscription(ctx context.Context, orgID string) (*Response, error)
	GetPlans(ctx context.Context) (*Response, error)
	GetTaxIDTypes(ctx context.Context) (*Response, error)
	CreateOrganisation(ctx context.Context, orgData interface{}) (*Response, error)
	GetOrganisation(ctx context.Context, orgID string) (*Response, error)
	UpdateOrganisation(ctx context.Context, orgID string, orgData interface{}) (*Response, error)
	UpdateOrganisationTaxID(ctx context.Context, orgID string, taxData interface{}) (*Response, error)
	UpdateOrganisationAddress(ctx context.Context, orgID string, addressData interface{}) (*Response, error)
	GetSubscriptions(ctx context.Context, orgID string) (*Response, error)
	CreateSubscription(ctx context.Context, orgID string, subData interface{}) (*Response, error)
	UpdateSubscription(ctx context.Context, orgID string, subData interface{}) (*Response, error)
	DeleteSubscription(ctx context.Context, orgID string) (*Response, error)
	GetSetupIntent(ctx context.Context, orgID string) (*Response, error)
	CreateSetupIntent(ctx context.Context, orgID string, setupIntentData interface{}) (*Response, error)
	CreatePaymentMethod(ctx context.Context, orgID string, pmData interface{}) (*Response, error)
	UpdatePaymentMethod(ctx context.Context, orgID, pmID string, pmData interface{}) (*Response, error)
	DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response, error)
	GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response, error)
	DownloadInvoice(ctx context.Context, orgID, invoiceID string) ([]byte, error)
	CreateBillingPaymentMethod(ctx context.Context, pmData interface{}) (*Response, error)
	UpdateBillingAddress(ctx context.Context, addressData interface{}) (*Response, error)
	UpdateBillingTaxID(ctx context.Context, taxData interface{}) (*Response, error)
}

type HTTPClient struct {
	httpClient *http.Client
	config     config.BillingConfiguration
}

type Response struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewClient(cfg config.BillingConfiguration) *HTTPClient {
	return &HTTPClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: cfg,
	}
}

func (c *HTTPClient) HealthCheck(ctx context.Context) error {
	if !c.config.Enabled {
		return fmt.Errorf("billing is not enabled")
	}

	if c.config.URL == "" {
		return fmt.Errorf("billing service URL is not configured")
	}

	// Make a simple health check request to the billing service
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/up", c.config.URL), nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	if c.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to billing service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("billing service health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (c *HTTPClient) GetUsage(ctx context.Context, orgID string) (*Response, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/usage", orgID), nil)
}

func (c *HTTPClient) GetInvoices(ctx context.Context, orgID string) (*Response, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/invoices", orgID), nil)
}

func (c *HTTPClient) GetPaymentMethods(ctx context.Context, orgID string) (*Response, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/payment_methods", orgID), nil)
}

func (c *HTTPClient) GetSubscription(ctx context.Context, orgID string) (*Response, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/subscriptions", orgID), nil)
}

func (c *HTTPClient) GetPlans(ctx context.Context) (*Response, error) {
	return c.makeRequest(ctx, "GET", "/plans", nil)
}

func (c *HTTPClient) GetTaxIDTypes(ctx context.Context) (*Response, error) {
	return c.makeRequest(ctx, "GET", "/tax_id_types", nil)
}

// Organisation methods
func (c *HTTPClient) CreateOrganisation(ctx context.Context, orgData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "POST", "/organisations", orgData)
}

func (c *HTTPClient) GetOrganisation(ctx context.Context, orgID string) (*Response, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s", orgID), nil)
}

func (c *HTTPClient) UpdateOrganisation(ctx context.Context, orgID string, orgData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "PUT", fmt.Sprintf("/organisations/%s", orgID), orgData)
}

func (c *HTTPClient) UpdateOrganisationTaxID(ctx context.Context, orgID string, taxData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "POST", fmt.Sprintf("/organisations/%s/tax_id", orgID), taxData)
}

func (c *HTTPClient) UpdateOrganisationAddress(ctx context.Context, orgID string, addressData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "POST", fmt.Sprintf("/organisations/%s/address", orgID), addressData)
}

// Subscription methods
func (c *HTTPClient) GetSubscriptions(ctx context.Context, orgID string) (*Response, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/subscriptions", orgID), nil)
}

func (c *HTTPClient) CreateSubscription(ctx context.Context, orgID string, subData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "POST", fmt.Sprintf("/organisations/%s/subscriptions", orgID), subData)
}

func (c *HTTPClient) UpdateSubscription(ctx context.Context, orgID string, subData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "PUT", fmt.Sprintf("/organisations/%s/subscriptions", orgID), subData)
}

func (c *HTTPClient) DeleteSubscription(ctx context.Context, orgID string) (*Response, error) {
	return c.makeRequest(ctx, "DELETE", fmt.Sprintf("/organisations/%s/subscriptions", orgID), nil)
}

// Payment method methods
func (c *HTTPClient) GetSetupIntent(ctx context.Context, orgID string) (*Response, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/payment_methods/setup_intent", orgID), nil)
}

func (c *HTTPClient) CreateSetupIntent(ctx context.Context, orgID string, setupIntentData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "POST", fmt.Sprintf("/organisations/%s/payment_methods/setup_intent", orgID), setupIntentData)
}

func (c *HTTPClient) CreatePaymentMethod(ctx context.Context, orgID string, pmData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "POST", fmt.Sprintf("/organisations/%s/payment_methods", orgID), pmData)
}

func (c *HTTPClient) UpdatePaymentMethod(ctx context.Context, orgID, pmID string, pmData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "PUT", fmt.Sprintf("/organisations/%s/payment_methods/%s", orgID, pmID), pmData)
}

func (c *HTTPClient) DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response, error) {
	return c.makeRequest(ctx, "DELETE", fmt.Sprintf("/organisations/%s/payment_methods/%s", orgID, pmID), nil)
}

// Invoice methods
func (c *HTTPClient) GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/invoices/%s", orgID, invoiceID), nil)
}

func (c *HTTPClient) DownloadInvoice(ctx context.Context, orgID, invoiceID string) ([]byte, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("billing is not enabled")
	}

	url := fmt.Sprintf("%s/organisations/%s/invoices/%s/download", c.config.URL, orgID, invoiceID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	if c.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download invoice: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Read the PDF content
	pdfContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF content: %w", err)
	}

	return pdfContent, nil
}

// Public billing methods
func (c *HTTPClient) CreateBillingPaymentMethod(ctx context.Context, pmData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "POST", "/billing/payment-method", pmData)
}

func (c *HTTPClient) UpdateBillingAddress(ctx context.Context, addressData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "POST", "/billing/address", addressData)
}

func (c *HTTPClient) UpdateBillingTaxID(ctx context.Context, taxData interface{}) (*Response, error) {
	return c.makeRequest(ctx, "POST", "/billing/tax-id", taxData)
}

func (c *HTTPClient) makeRequest(ctx context.Context, method, path string, body interface{}) (*Response, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("billing is not enabled")
	}

	// Add /api/v1 prefix for Overwatch compatibility
	if !strings.HasPrefix(path, "/api/v1") && !strings.HasPrefix(path, "/billing") {
		path = "/api/v1" + path
	}

	url := fmt.Sprintf("%s%s", c.config.URL, path)

	var req *http.Request
	var err error

	if body != nil {
		jsonBody, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", marshalErr)
		}
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(jsonBody))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to billing service: %w", err)
	}
	defer resp.Body.Close()

	var billingResp Response
	if err := json.NewDecoder(resp.Body).Decode(&billingResp); err != nil {
		return nil, fmt.Errorf("failed to read billing response: %w", err)
	}

	// If the billing service returned an error response, return it as an error
	if !billingResp.Status {
		return &billingResp, fmt.Errorf("billing service error: %s", billingResp.Message)
	}

	return &billingResp, nil
}
