package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

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
	GetOrganisationLicense(ctx context.Context, orgID string) (*Response[OrganisationLicense], error)
	GetOrganisation(ctx context.Context, orgID string) (*Response[BillingOrganisation], error)
	GetWorkspaceConfigBySlug(ctx context.Context, slug string) (*Response[WorkspaceConfigData], error)
	UpdateOrganisation(ctx context.Context, orgID string, orgData BillingOrganisation) (*Response[BillingOrganisation], error)
	UpdateOrganisationTaxID(ctx context.Context, orgID string, taxData UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error)
	UpdateOrganisationAddress(ctx context.Context, orgID string, addressData UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error)
	GetSubscriptions(ctx context.Context, orgID string) (*Response[[]BillingSubscription], error)
	OnboardSubscription(ctx context.Context, orgID string, req OnboardSubscriptionRequest) (*Response[Checkout], error)
	UpgradeSubscription(ctx context.Context, orgID, subscriptionID string, req UpgradeSubscriptionRequest) (*Response[Checkout], error)
	DeleteSubscription(ctx context.Context, orgID, subscriptionID string) (*Response[interface{}], error)
	StartGuestCheckout(ctx context.Context, req StartGuestCheckoutRequest) (*Response[Checkout], error)
	CompleteGuestCheckout(ctx context.Context, req CompleteGuestCheckoutRequest) (*Response[GuestCheckoutCompletion], error)
	GetSelfHostedSubscription(ctx context.Context, licenseKey string) (*Response[BillingSubscription], error)
	DeleteSelfHostedSubscription(ctx context.Context, licenseKey string) (*Response[interface{}], error)
	GetSelfHostedOrganisation(ctx context.Context, licenseKey string) (*Response[BillingOrganisation], error)
	UpdateSelfHostedOrganisationTaxID(ctx context.Context, licenseKey string, taxData UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error)
	UpdateSelfHostedOrganisationAddress(ctx context.Context, licenseKey string, addressData UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error)
	GetSelfHostedInvoices(ctx context.Context, licenseKey string) (*Response[[]Invoice], error)
	GetSelfHostedInvoice(ctx context.Context, licenseKey, invoiceID string) (*Response[Invoice], error)
	DownloadSelfHostedInvoice(ctx context.Context, licenseKey, invoiceID string) (*http.Response, error)
	GetSelfHostedPaymentMethods(ctx context.Context, licenseKey string) (*Response[[]PaymentMethod], error)
	GetSelfHostedSetupIntent(ctx context.Context, licenseKey string) (*Response[SetupIntent], error)
	DeactivateOrganisation(ctx context.Context, orgID string) error
	GetSetupIntent(ctx context.Context, orgID string) (*Response[SetupIntent], error)
	CreateSetupIntent(ctx context.Context, orgID string, setupIntentData CreateSetupIntentRequest) (*Response[SetupIntent], error)
	GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response[Invoice], error)
	DownloadInvoice(ctx context.Context, orgID, invoiceID string) (*http.Response, error)
	SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error)
	DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error)
	SetDefaultSelfHostedPaymentMethod(ctx context.Context, licenseKey, pmID string) (*Response[interface{}], error)
	DeleteSelfHostedPaymentMethod(ctx context.Context, licenseKey, pmID string) (*Response[interface{}], error)
}

// headerLicenseKey is the request header carrying the self-hosted license proof.
const headerLicenseKey = "X-Convoy-License-Key"

type HTTPClient struct {
	httpClient *http.Client
	config     config.BillingConfiguration
}

// setBillingAuthHeader sets the bearer Authorization header when an API key is configured.
func setBillingAuthHeader(req *http.Request, cfg config.BillingConfiguration) {
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))
	}
}

type Response[T any] struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
}

func NewClient(cfg config.BillingConfiguration) *HTTPClient {
	return &HTTPClient{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		config: cfg,
	}
}

func (c *HTTPClient) HealthCheck(ctx context.Context) error {
	if c.config.URL == "" {
		return fmt.Errorf("billing service URL is not configured")
	}

	// Make a simple health check request to the billing service
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/up", c.config.URL), http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	setBillingAuthHeader(req, c.config)

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
	if strings.TrimSpace(c.config.APIKey) == "" {
		return makeRequest[[]Plan](ctx, c.httpClient, c.config, "GET", "/public/self_hosted/plans", nil)
	}
	return makeRequest[[]Plan](ctx, c.httpClient, c.config, "GET", "/plans", nil)
}

func (c *HTTPClient) GetTaxIDTypes(ctx context.Context) (*Response[[]TaxIDType], error) {
	url := fmt.Sprintf("%s/tax_id_types", c.config.URL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	setBillingAuthHeader(req, c.config)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to billing service: %w", err)
	}
	defer resp.Body.Close()

	// NOTE: /tax_id_types returns a bare JSON array, not the {status,message,data}
	// envelope every other endpoint uses, so this method intentionally does not route
	// through makeRequestWithHeaders. See overwatch TaxIdTypesController#index.
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

func (c *HTTPClient) GetOrganisationLicense(ctx context.Context, orgID string) (*Response[OrganisationLicense], error) {
	return makeRequest[OrganisationLicense](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s/license", orgID), nil)
}

func (c *HTTPClient) GetOrganisation(ctx context.Context, orgID string) (*Response[BillingOrganisation], error) {
	return makeRequest[BillingOrganisation](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s", orgID), nil)
}

func (c *HTTPClient) GetWorkspaceConfigBySlug(ctx context.Context, slug string) (*Response[WorkspaceConfigData], error) {
	if slug == "" {
		return nil, fmt.Errorf("slug is required")
	}
	path := fmt.Sprintf("/api/v1/workspace_config?slug=%s", strings.ReplaceAll(url.QueryEscape(slug), "+", "%20"))
	return makeRequest[WorkspaceConfigData](ctx, c.httpClient, c.config, "GET", path, nil)
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

func (c *HTTPClient) StartGuestCheckout(ctx context.Context, req StartGuestCheckoutRequest) (*Response[Checkout], error) {
	return makeRequest[Checkout](ctx, c.httpClient, c.config, "POST", "/public/self_hosted_checkouts/start", req)
}

func (c *HTTPClient) CompleteGuestCheckout(ctx context.Context, req CompleteGuestCheckoutRequest) (*Response[GuestCheckoutCompletion], error) {
	return makeRequest[GuestCheckoutCompletion](ctx, c.httpClient, c.config, "POST", "/public/self_hosted_checkouts/complete", req)
}

func (c *HTTPClient) GetSelfHostedSubscription(ctx context.Context, licenseKey string) (*Response[BillingSubscription], error) {
	return makeLicenseRequest[BillingSubscription](ctx, c.httpClient, c.config, "GET", "/public/self_hosted_billing/subscription", licenseKey)
}

func (c *HTTPClient) DeleteSelfHostedSubscription(ctx context.Context, licenseKey string) (*Response[interface{}], error) {
	return makeLicenseRequest[interface{}](ctx, c.httpClient, c.config, "DELETE", "/public/self_hosted_billing/subscription", licenseKey)
}

func (c *HTTPClient) GetSelfHostedOrganisation(ctx context.Context, licenseKey string) (*Response[BillingOrganisation], error) {
	return makeLicenseRequest[BillingOrganisation](ctx, c.httpClient, c.config, "GET", "/public/self_hosted_billing/organisation", licenseKey)
}

func (c *HTTPClient) UpdateSelfHostedOrganisationTaxID(ctx context.Context, licenseKey string, taxData UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error) {
	return makeLicenseRequestWithBody[BillingOrganisation](ctx, c.httpClient, c.config, "PUT", "/public/self_hosted_billing/tax_id", taxData, licenseKey)
}

func (c *HTTPClient) UpdateSelfHostedOrganisationAddress(ctx context.Context, licenseKey string, addressData UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error) {
	return makeLicenseRequestWithBody[BillingOrganisation](ctx, c.httpClient, c.config, "PUT", "/public/self_hosted_billing/billing_address", addressData, licenseKey)
}

func (c *HTTPClient) GetSelfHostedInvoices(ctx context.Context, licenseKey string) (*Response[[]Invoice], error) {
	return makeLicenseRequest[[]Invoice](ctx, c.httpClient, c.config, "GET", "/public/self_hosted_billing/invoices", licenseKey)
}

func (c *HTTPClient) GetSelfHostedInvoice(ctx context.Context, licenseKey, invoiceID string) (*Response[Invoice], error) {
	return makeLicenseRequest[Invoice](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/public/self_hosted_billing/invoices/%s", url.PathEscape(invoiceID)), licenseKey)
}

func (c *HTTPClient) DownloadSelfHostedInvoice(ctx context.Context, licenseKey, invoiceID string) (*http.Response, error) {
	invoiceResp, err := c.GetSelfHostedInvoice(ctx, licenseKey, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	// Download the rendered PDF only. The hosted_link is an HTML page, not a PDF,
	// so we never fall back to it; a missing pdf_link is a clear, surfaced error.
	pdfLink := invoiceResp.Data.PDFLink
	if pdfLink == "" {
		return nil, fmt.Errorf("invoice PDF link not found")
	}

	return c.downloadPDF(ctx, pdfLink, false)
}

func (c *HTTPClient) GetSelfHostedPaymentMethods(ctx context.Context, licenseKey string) (*Response[[]PaymentMethod], error) {
	return makeLicenseRequest[[]PaymentMethod](ctx, c.httpClient, c.config, "GET", "/public/self_hosted_billing/payment_methods", licenseKey)
}

func (c *HTTPClient) GetSelfHostedSetupIntent(ctx context.Context, licenseKey string) (*Response[SetupIntent], error) {
	return makeLicenseRequest[SetupIntent](ctx, c.httpClient, c.config, "GET", "/public/self_hosted_billing/payment_methods/setup_intent", licenseKey)
}

func (c *HTTPClient) DeactivateOrganisation(ctx context.Context, orgID string) error {
	_, err := makeRequest[interface{}](ctx, c.httpClient, c.config, "POST", fmt.Sprintf("/organisations/%s/deactivate", orgID), nil)
	return err
}

func (c *HTTPClient) DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	return makeRequest[interface{}](ctx, c.httpClient, c.config, "DELETE", fmt.Sprintf("/organisations/%s/payment_methods/%s", orgID, pmID), nil)
}

func (c *HTTPClient) SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	return makeRequest[interface{}](ctx, c.httpClient, c.config, "PATCH", fmt.Sprintf("/organisations/%s/payment_methods/%s/default", orgID, pmID), nil)
}

func (c *HTTPClient) SetDefaultSelfHostedPaymentMethod(ctx context.Context, licenseKey, pmID string) (*Response[interface{}], error) {
	return makeLicenseRequest[interface{}](ctx, c.httpClient, c.config, "PATCH", fmt.Sprintf("/public/self_hosted_billing/payment_methods/%s/default", url.PathEscape(pmID)), licenseKey)
}

func (c *HTTPClient) DeleteSelfHostedPaymentMethod(ctx context.Context, licenseKey, pmID string) (*Response[interface{}], error) {
	return makeLicenseRequest[interface{}](ctx, c.httpClient, c.config, "DELETE", fmt.Sprintf("/public/self_hosted_billing/payment_methods/%s", url.PathEscape(pmID)), licenseKey)
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
	invoiceResp, err := c.GetInvoice(ctx, orgID, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	pdfLink := invoiceResp.Data.PDFLink
	if pdfLink == "" {
		return nil, fmt.Errorf("invoice PDF link not found")
	}

	return c.downloadPDF(ctx, pdfLink, true)
}

// downloadPDF resolves a billing PDF link and streams it back. When auth is true the
// configured billing API key is sent as a bearer token (cloud invoices); self-hosted
// invoices use a pre-signed link and pass auth=false.
func (c *HTTPClient) downloadPDF(ctx context.Context, pdfLink string, auth bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pdfLink, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF download request: %w", err)
	}

	if auth {
		setBillingAuthHeader(req, c.config)
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
	return makeRequestWithHeaders[T](ctx, httpClient, config, method, path, body, nil)
}

func makeLicenseRequest[T any](ctx context.Context, httpClient *http.Client, config config.BillingConfiguration, method, path, licenseKey string) (*Response[T], error) {
	return makeLicenseRequestWithBody[T](ctx, httpClient, config, method, path, nil, licenseKey)
}

func makeLicenseRequestWithBody[T any](ctx context.Context, httpClient *http.Client, config config.BillingConfiguration, method, path string, body interface{}, licenseKey string) (*Response[T], error) {
	licenseKey = strings.TrimSpace(licenseKey)
	if licenseKey == "" {
		return nil, fmt.Errorf("self-hosted license key is required")
	}
	return makeRequestWithHeaders[T](ctx, httpClient, config, method, path, body, map[string]string{
		headerLicenseKey: licenseKey,
	})
}

func makeRequestWithHeaders[T any](ctx context.Context, httpClient *http.Client, config config.BillingConfiguration, method, path string, body interface{}, headers map[string]string) (*Response[T], error) {
	if strings.TrimSpace(config.URL) == "" {
		return nil, fmt.Errorf("billing service URL is not configured")
	}

	if !strings.HasPrefix(path, "/api/v1") && !strings.HasPrefix(path, "/billing") && !strings.HasPrefix(path, "/public") {
		path = "/api/v1" + path
	}

	url := fmt.Sprintf("%s%s", config.URL, path)
	isPublicPath := strings.HasPrefix(path, "/public")

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
	if !isPublicPath {
		setBillingAuthHeader(req, config)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to billing service: %w", err)
	}
	defer resp.Body.Close()

	rawResp, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read billing response body: %w", readErr)
	}

	var baseResp struct {
		Status  bool            `json:"status"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	}
	if err := json.Unmarshal(rawResp, &baseResp); err != nil {
		return nil, fmt.Errorf("failed to read billing response: %w", err)
	}

	if !baseResp.Status {
		// Typed error carries the upstream status so callers map it without string matching.
		msg := baseResp.Message
		if msg == "" {
			msg = fmt.Sprintf("billing service returned error status: %d", resp.StatusCode)
		}
		return nil, &Error{StatusCode: resp.StatusCode, Message: msg}
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
