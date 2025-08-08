package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/config"
)

type Client struct {
	httpClient *http.Client
	config     config.BillingConfiguration
}

type BillingResponse struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewClient(cfg config.BillingConfiguration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: cfg,
	}
}

func (c *Client) HealthCheck(ctx context.Context) error {
	if !c.config.Enabled {
		return fmt.Errorf("billing is not enabled")
	}

	if c.config.URL == "" {
		return fmt.Errorf("billing service URL is not configured")
	}

	// Make a simple health check request to the billing service
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/health", c.config.URL), nil)
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

func (c *Client) GetUsage(ctx context.Context, orgID string) (*BillingResponse, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/usage", orgID), nil)
}

func (c *Client) GetInvoices(ctx context.Context, orgID string) (*BillingResponse, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/invoices", orgID), nil)
}

func (c *Client) GetPaymentMethods(ctx context.Context, orgID string) (*BillingResponse, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/payment_methods", orgID), nil)
}

func (c *Client) GetSubscription(ctx context.Context, orgID string) (*BillingResponse, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/subscription", orgID), nil)
}

func (c *Client) GetPlans(ctx context.Context) (*BillingResponse, error) {
	return c.makeRequest(ctx, "GET", "/plans", nil)
}

func (c *Client) GetTaxIDTypes(ctx context.Context) (*BillingResponse, error) {
	return c.makeRequest(ctx, "GET", "/tax_id_types", nil)
}

// Organisation methods
func (c *Client) CreateOrganisation(ctx context.Context, orgData interface{}) (*BillingResponse, error) {
	return c.makeRequest(ctx, "POST", "/organisations", orgData)
}

func (c *Client) GetOrganisation(ctx context.Context, orgID string) (*BillingResponse, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s", orgID), nil)
}

func (c *Client) UpdateOrganisation(ctx context.Context, orgID string, orgData interface{}) (*BillingResponse, error) {
	return c.makeRequest(ctx, "PUT", fmt.Sprintf("/organisations/%s", orgID), orgData)
}

func (c *Client) UpdateOrganisationTaxID(ctx context.Context, orgID string, taxData interface{}) (*BillingResponse, error) {
	return c.makeRequest(ctx, "POST", fmt.Sprintf("/organisations/%s/tax_id", orgID), taxData)
}

func (c *Client) UpdateOrganisationAddress(ctx context.Context, orgID string, addressData interface{}) (*BillingResponse, error) {
	return c.makeRequest(ctx, "POST", fmt.Sprintf("/organisations/%s/address", orgID), addressData)
}

// Subscription methods
func (c *Client) GetSubscriptions(ctx context.Context, orgID string) (*BillingResponse, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/subscriptions", orgID), nil)
}

func (c *Client) CreateSubscription(ctx context.Context, orgID string, subData interface{}) (*BillingResponse, error) {
	return c.makeRequest(ctx, "POST", fmt.Sprintf("/organisations/%s/subscriptions", orgID), subData)
}

func (c *Client) UpdateSubscription(ctx context.Context, orgID string, subData interface{}) (*BillingResponse, error) {
	return c.makeRequest(ctx, "PUT", fmt.Sprintf("/organisations/%s/subscriptions", orgID), subData)
}

func (c *Client) DeleteSubscription(ctx context.Context, orgID string) (*BillingResponse, error) {
	return c.makeRequest(ctx, "DELETE", fmt.Sprintf("/organisations/%s/subscriptions", orgID), nil)
}

// Payment method methods
func (c *Client) GetSetupIntent(ctx context.Context, orgID string) (*BillingResponse, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/payment_methods/setup_intent", orgID), nil)
}

func (c *Client) CreatePaymentMethod(ctx context.Context, orgID string, pmData interface{}) (*BillingResponse, error) {
	return c.makeRequest(ctx, "POST", fmt.Sprintf("/organisations/%s/payment_methods", orgID), pmData)
}

func (c *Client) DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*BillingResponse, error) {
	return c.makeRequest(ctx, "DELETE", fmt.Sprintf("/organisations/%s/payment_methods/%s", orgID, pmID), nil)
}

// Invoice methods
func (c *Client) GetInvoice(ctx context.Context, orgID, invoiceID string) (*BillingResponse, error) {
	return c.makeRequest(ctx, "GET", fmt.Sprintf("/organisations/%s/invoices/%s", orgID, invoiceID), nil)
}

func (c *Client) DownloadInvoice(ctx context.Context, orgID, invoiceID string) ([]byte, error) {
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
func (c *Client) CreateBillingPaymentMethod(ctx context.Context, pmData interface{}) (*BillingResponse, error) {
	return c.makeRequest(ctx, "POST", "/billing/payment-method", pmData)
}

func (c *Client) UpdateBillingAddress(ctx context.Context, addressData interface{}) (*BillingResponse, error) {
	return c.makeRequest(ctx, "POST", "/billing/address", addressData)
}

func (c *Client) UpdateBillingTaxID(ctx context.Context, taxData interface{}) (*BillingResponse, error) {
	return c.makeRequest(ctx, "POST", "/billing/tax-id", taxData)
}

func (c *Client) makeRequest(ctx context.Context, method, path string, body interface{}) (*BillingResponse, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("billing is not enabled")
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

	var billingResp BillingResponse
	if err := json.NewDecoder(resp.Body).Decode(&billingResp); err != nil {
		return nil, fmt.Errorf("failed to read billing response: %w", err)
	}

	// If the billing service returned an error response, return it as an error
	if !billingResp.Status {
		return &billingResp, fmt.Errorf("billing service error: %s", billingResp.Message)
	}

	return &billingResp, nil
}
