package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy/config"
)

type Client interface {
	HealthCheck(ctx context.Context) error
	GetUsage(ctx context.Context, orgID string) (*Response[Usage], error)
	GetInvoices(ctx context.Context, orgID string) (*Response[[]Invoice], error)
	GetPaymentMethods(ctx context.Context, orgID string) (*Response[[]PaymentMethod], error)
	GetSubscription(ctx context.Context, orgID string) (*Response[BillingSubscription], error)
	GetPlans(ctx context.Context) (*Response[[]Plan], error)
	GetTaxIDTypes(ctx context.Context) (*Response[[]TaxIDType], error)
	CreateOrganisation(ctx context.Context, orgData BillingOrganisation) (*Response[BillingOrganisation], error)
	GetOrganisation(ctx context.Context, orgID string) (*Response[BillingOrganisation], error)
	UpdateOrganisation(ctx context.Context, orgID string, orgData BillingOrganisation) (*Response[BillingOrganisation], error)
	UpdateOrganisationTaxID(ctx context.Context, orgID string, taxData UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error)
	UpdateOrganisationAddress(ctx context.Context, orgID string, addressData UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error)
	GetSubscriptions(ctx context.Context, orgID string) (*Response[[]BillingSubscription], error)
	OnboardSubscription(ctx context.Context, orgID string, req OnboardSubscriptionRequest) (*Response[Checkout], error)
	UpgradeSubscription(ctx context.Context, orgID, subscriptionID string, req UpgradeSubscriptionRequest) (*Response[Checkout], error)
	DeleteSubscription(ctx context.Context, orgID, subscriptionID string) (*Response[interface{}], error)
	GetSetupIntent(ctx context.Context, orgID string) (*Response[SetupIntent], error)
	CreateSetupIntent(ctx context.Context, orgID string, setupIntentData CreateSetupIntentRequest) (*Response[SetupIntent], error)
	GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response[Invoice], error)
	DownloadInvoice(ctx context.Context, orgID, invoiceID string) (*http.Response, error)
	SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error)
	DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error)
}

type HTTPClient struct {
	httpClient *http.Client
	config     config.BillingConfiguration
}

type Response[T any] struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
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
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/up", c.config.URL), http.NoBody)
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

func (c *HTTPClient) GetUsage(ctx context.Context, orgID string) (*Response[Usage], error) {
	return makeRequest[Usage](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s/usage", orgID), nil)
}

func (c *HTTPClient) GetInvoices(ctx context.Context, orgID string) (*Response[[]Invoice], error) {
	return makeRequest[[]Invoice](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s/invoices", orgID), nil)
}

func (c *HTTPClient) GetPaymentMethods(ctx context.Context, orgID string) (*Response[[]PaymentMethod], error) {
	return makeRequest[[]PaymentMethod](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s/payment_methods", orgID), nil)
}

func (c *HTTPClient) GetSubscription(ctx context.Context, orgID string) (*Response[BillingSubscription], error) {
	return makeRequest[BillingSubscription](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s/subscriptions", orgID), nil)
}

func (c *HTTPClient) GetPlans(ctx context.Context) (*Response[[]Plan], error) {
	return makeRequest[[]Plan](ctx, c.httpClient, c.config, "GET", "/plans", nil)
}

func (c *HTTPClient) GetTaxIDTypes(ctx context.Context) (*Response[[]TaxIDType], error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("billing is not enabled")
	}

	url := fmt.Sprintf("%s/tax_id_types", c.config.URL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
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

	var taxIdTypes []TaxIDType
	if err := json.NewDecoder(resp.Body).Decode(&taxIdTypes); err != nil {
		return nil, fmt.Errorf("failed to read billing response: %w", err)
	}

	return &Response[[]TaxIDType]{
		Status:  true,
		Message: "Tax ID types retrieved successfully",
		Data:    taxIdTypes,
	}, nil
}

func (c *HTTPClient) CreateOrganisation(ctx context.Context, orgData BillingOrganisation) (*Response[BillingOrganisation], error) {
	return makeRequest[BillingOrganisation](ctx, c.httpClient, c.config, "POST", "/organisations", orgData)
}

func (c *HTTPClient) GetOrganisation(ctx context.Context, orgID string) (*Response[BillingOrganisation], error) {
	return makeRequest[BillingOrganisation](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s", orgID), nil)
}

func (c *HTTPClient) UpdateOrganisation(ctx context.Context, orgID string, orgData BillingOrganisation) (*Response[BillingOrganisation], error) {
	return makeRequest[BillingOrganisation](ctx, c.httpClient, c.config, "PUT", fmt.Sprintf("/organisations/%s", orgID), orgData)
}

func (c *HTTPClient) UpdateOrganisationTaxID(ctx context.Context, orgID string, taxData UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error) {
	return makeRequest[BillingOrganisation](ctx, c.httpClient, c.config, "PUT", fmt.Sprintf("/organisations/%s/tax_id", orgID), taxData)
}

func (c *HTTPClient) UpdateOrganisationAddress(ctx context.Context, orgID string, addressData UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error) {
	return makeRequest[BillingOrganisation](ctx, c.httpClient, c.config, "PUT", fmt.Sprintf("/organisations/%s/billing_address", orgID), addressData)
}

func (c *HTTPClient) GetSubscriptions(ctx context.Context, orgID string) (*Response[[]BillingSubscription], error) {
	return makeRequest[[]BillingSubscription](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s/subscriptions", orgID), nil)
}

func (c *HTTPClient) OnboardSubscription(ctx context.Context, orgID string, req OnboardSubscriptionRequest) (*Response[Checkout], error) {
	return makeRequest[Checkout](ctx, c.httpClient, c.config, "POST", fmt.Sprintf("/organisations/%s/subscriptions/onboard", orgID), req)
}

func (c *HTTPClient) UpgradeSubscription(ctx context.Context, orgID, subscriptionID string, req UpgradeSubscriptionRequest) (*Response[Checkout], error) {
	return makeRequest[Checkout](ctx, c.httpClient, c.config, "PUT", fmt.Sprintf("/organisations/%s/subscriptions/%s/upgrade", orgID, subscriptionID), req)
}

func (c *HTTPClient) DeleteSubscription(ctx context.Context, orgID, subscriptionID string) (*Response[interface{}], error) {
	return makeRequest[interface{}](ctx, c.httpClient, c.config, "DELETE", fmt.Sprintf("/organisations/%s/subscriptions/%s", orgID, subscriptionID), nil)
}

func (c *HTTPClient) DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	return makeRequest[interface{}](ctx, c.httpClient, c.config, "DELETE", fmt.Sprintf("/organisations/%s/payment_methods/%s", orgID, pmID), nil)
}

func (c *HTTPClient) SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	return makeRequest[interface{}](ctx, c.httpClient, c.config, "PATCH", fmt.Sprintf("/organisations/%s/payment_methods/%s/default", orgID, pmID), nil)
}

func (c *HTTPClient) GetSetupIntent(ctx context.Context, orgID string) (*Response[SetupIntent], error) {
	return makeRequest[SetupIntent](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s/payment_methods/setup_intent", orgID), nil)
}

func (c *HTTPClient) CreateSetupIntent(ctx context.Context, orgID string, setupIntentData CreateSetupIntentRequest) (*Response[SetupIntent], error) {
	return makeRequest[SetupIntent](ctx, c.httpClient, c.config, "POST", fmt.Sprintf("/organisations/%s/payment_methods/setup_intent", orgID), setupIntentData)
}

func (c *HTTPClient) GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response[Invoice], error) {
	return makeRequest[Invoice](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s/invoices/%s", orgID, invoiceID), nil)
}

func (c *HTTPClient) DownloadInvoice(ctx context.Context, orgID, invoiceID string) (*http.Response, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("billing is not enabled")
	}

	invoiceResp, err := c.GetInvoice(ctx, orgID, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	pdfLink := invoiceResp.Data.PDFLink
	if pdfLink == "" {
		return nil, fmt.Errorf("invoice PDF link not found")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", pdfLink, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF download request: %w", err)
	}

	if c.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download PDF from billing service: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("billing service returned error status: %d", resp.StatusCode)
	}

	return resp, nil
}

func makeRequest[T any](ctx context.Context, httpClient *http.Client, config config.BillingConfiguration, method, path string, body interface{}) (*Response[T], error) {
	if !config.Enabled {
		return nil, fmt.Errorf("billing is not enabled")
	}

	if !strings.HasPrefix(path, "/api/v1") && !strings.HasPrefix(path, "/billing") {
		path = "/api/v1" + path
	}

	url := fmt.Sprintf("%s%s", config.URL, path)

	var req *http.Request
	var err error

	if body != nil {
		jsonBody, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", marshalErr)
		}
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(jsonBody))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, http.NoBody)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.APIKey))
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to billing service: %w", err)
	}
	defer resp.Body.Close()

	var baseResp struct {
		Status  bool            `json:"status"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&baseResp); err != nil {
		return nil, fmt.Errorf("failed to read billing response: %w", err)
	}

	if !baseResp.Status {
		return nil, fmt.Errorf("billing service error: %s", baseResp.Message)
	}

	var data T
	if len(baseResp.Data) > 0 {
		if err := json.Unmarshal(baseResp.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response data: %w", err)
		}
	}

	return &Response[T]{
		Status:  baseResp.Status,
		Message: baseResp.Message,
		Data:    data,
	}, nil
}
