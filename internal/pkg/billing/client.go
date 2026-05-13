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

func billingNonJSONError(resp *http.Response, raw []byte, err error) error {
	status := 0
	ct := ""
	if resp != nil {
		status = resp.StatusCode
		ct = resp.Header.Get("Content-Type")
	}
	preview := strings.TrimSpace(string(raw))
	if preview == "" {
		preview = "(empty body)"
	} else if len(preview) > 280 {
		preview = preview[:280] + "..."
	}
	return &ServiceError{
		StatusCode: http.StatusBadGateway,
		Message:    fmt.Sprintf("billing returned non-JSON response (HTTP %d, Content-Type %q, body prefix: %q)", status, ct, preview),
	}
}

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
	DeactivateOrganisation(ctx context.Context, orgID string) error
	GetSetupIntent(ctx context.Context, orgID string) (*Response[SetupIntent], error)
	CreateSetupIntent(ctx context.Context, orgID string, setupIntentData CreateSetupIntentRequest) (*Response[SetupIntent], error)
	GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response[Invoice], error)
	DownloadInvoice(ctx context.Context, orgID, invoiceID string) (*http.Response, error)
	SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error)
	DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error)

	SelfHostedRegisterEmail(ctx context.Context, req SelfHostedRegisterEmailRequest) (*Response[SelfHostedRegisterEmailData], error)
	SelfHostedVerifyEmail(ctx context.Context, code string) (*Response[SelfHostedVerifyEmailData], error)
	SelfHostedStartCheckout(ctx context.Context, licenseKey string, req SelfHostedStartCheckoutRequest) (*Response[Checkout], error)

	LicenseBillingGetPlans(ctx context.Context, licenseKey string) (*Response[[]Plan], error)
	LicenseBillingGetSubscription(ctx context.Context, licenseKey string) (*Response[BillingSubscription], error)
	LicenseBillingDeleteSubscription(ctx context.Context, licenseKey string) (*Response[interface{}], error)
	LicenseBillingGetPaymentMethods(ctx context.Context, licenseKey string) (*Response[[]PaymentMethod], error)
	LicenseBillingGetSetupIntent(ctx context.Context, licenseKey string) (*Response[SetupIntent], error)
	LicenseBillingSetDefaultPaymentMethod(ctx context.Context, licenseKey, pmID string) (*Response[interface{}], error)
	LicenseBillingDeletePaymentMethod(ctx context.Context, licenseKey, pmID string) (*Response[interface{}], error)
	LicenseBillingGetInvoices(ctx context.Context, licenseKey string) (*Response[[]Invoice], error)
	LicenseBillingGetInvoice(ctx context.Context, licenseKey, invoiceID string) (*Response[Invoice], error)
	LicenseBillingRecoverExternalID(ctx context.Context, licenseKey, newExternalID string) (*Response[LicenseRecoverExternalIDData], error)
	LicenseBillingGetContext(ctx context.Context, licenseKey string) (*Response[LicenseBillingContextData], error)
	LicenseBillingGetOrganisation(ctx context.Context, licenseKey string) (*Response[BillingOrganisation], error)
	LicenseBillingUpdateOrganisation(ctx context.Context, licenseKey string, orgData BillingOrganisation) (*Response[BillingOrganisation], error)
	LicenseBillingUpdateOrganisationAddress(ctx context.Context, licenseKey string, addressData UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error)
	LicenseBillingUpdateOrganisationTaxID(ctx context.Context, licenseKey string, taxData UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error)
	LicenseBillingGetTaxIDTypes(ctx context.Context, licenseKey string) (*Response[[]TaxIDType], error)
	LicenseBillingUpgradeSubscription(ctx context.Context, licenseKey string, req UpgradeSubscriptionRequest) (*Response[Checkout], error)
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

type ServiceError struct {
	StatusCode int
	Message    string
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("billing service error: %s", e.Message)
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
	resp, err := makeRequest[json.RawMessage](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s/subscriptions", orgID), nil)
	if err != nil {
		return nil, err
	}

	subscriptions, err := decodeSubscriptions(resp.Data)
	if err != nil {
		return nil, err
	}

	subscription := selectCurrentSubscription(subscriptions)
	return &Response[BillingSubscription]{Status: resp.Status, Message: resp.Message, Data: subscription}, nil
}

func (c *HTTPClient) GetPlans(ctx context.Context) (*Response[[]Plan], error) {
	return makeRequest[[]Plan](ctx, c.httpClient, c.config, "GET", "/plans", nil)
}

func (c *HTTPClient) GetTaxIDTypes(ctx context.Context) (*Response[[]TaxIDType], error) {
	base := strings.TrimSuffix(strings.TrimSpace(c.config.URL), "/")
	base = strings.TrimSuffix(base, "/api/v1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/tax_id_types", base), http.NoBody)
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

	rawResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read billing response body: %w", err)
	}

	var envelope struct {
		Status  bool            `json:"status"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	}
	if err := json.Unmarshal(rawResp, &envelope); err == nil && (envelope.Message != "" || envelope.Data != nil) {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 || !envelope.Status {
			return nil, &ServiceError{StatusCode: resp.StatusCode, Message: envelope.Message}
		}
		var taxIDTypes []TaxIDType
		if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
			if err := json.Unmarshal(envelope.Data, &taxIDTypes); err != nil {
				return nil, fmt.Errorf("failed to unmarshal response data: %w", err)
			}
		}
		return &Response[[]TaxIDType]{Status: true, Message: envelope.Message, Data: taxIDTypes}, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, billingNonJSONError(resp, rawResp, fmt.Errorf("unexpected tax ID types response"))
	}

	var taxIDTypes []TaxIDType
	if err := json.Unmarshal(rawResp, &taxIDTypes); err != nil {
		return nil, billingNonJSONError(resp, rawResp, err)
	}
	return &Response[[]TaxIDType]{Status: true, Message: "Tax ID types retrieved successfully", Data: taxIDTypes}, nil
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
	resp, err := makeRequest[json.RawMessage](ctx, c.httpClient, c.config, "GET", fmt.Sprintf("/organisations/%s/subscriptions", orgID), nil)
	if err != nil {
		return nil, err
	}

	subscriptions, err := decodeSubscriptions(resp.Data)
	if err != nil {
		return nil, err
	}

	return &Response[[]BillingSubscription]{Status: resp.Status, Message: resp.Message, Data: subscriptions}, nil
}

func decodeSubscriptions(raw json.RawMessage) ([]BillingSubscription, error) {
	data := bytes.TrimSpace(raw)
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}

	if data[0] == '[' {
		var subscriptions []BillingSubscription
		if err := json.Unmarshal(data, &subscriptions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response data: %w", err)
		}
		return subscriptions, nil
	}

	var subscription BillingSubscription
	if err := json.Unmarshal(data, &subscription); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response data: %w", err)
	}
	if subscription.ID == "" {
		return nil, nil
	}
	return []BillingSubscription{subscription}, nil
}

func selectCurrentSubscription(subscriptions []BillingSubscription) BillingSubscription {
	for _, subscription := range subscriptions {
		if HasActiveSubscription(subscription) {
			return subscription
		}
	}
	if len(subscriptions) > 0 {
		return subscriptions[0]
	}
	return BillingSubscription{}
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

	req, err := http.NewRequestWithContext(ctx, "GET", pdfLink, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF download request: %w", err)
	}

	pdfURL, parsePDFErr := url.Parse(pdfLink)
	billingURL, parseBillingErr := url.Parse(c.config.URL)
	if c.config.APIKey != "" && parsePDFErr == nil && parseBillingErr == nil && strings.EqualFold(pdfURL.Host, billingURL.Host) {
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

func makeRequest[T any](ctx context.Context, httpClient *http.Client, cfg config.BillingConfiguration, method, path string, body interface{}) (*Response[T], error) {
	if !strings.HasPrefix(path, "/api/v1") && !strings.HasPrefix(path, "/billing") {
		path = "/api/v1" + path
	}

	url := fmt.Sprintf("%s%s", cfg.URL, path)

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
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))
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
		return nil, billingNonJSONError(resp, rawResp, err)
	}

	if !baseResp.Status {
		return nil, &ServiceError{StatusCode: resp.StatusCode, Message: baseResp.Message}
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

func (c *HTTPClient) SelfHostedRegisterEmail(ctx context.Context, req SelfHostedRegisterEmailRequest) (*Response[SelfHostedRegisterEmailData], error) {
	body := map[string]string{"email": req.Email}
	if org := strings.TrimSpace(req.OrganisationName); org != "" {
		body["organisation_name"] = org
	}
	return makeRequest[SelfHostedRegisterEmailData](ctx, c.httpClient, c.config, "POST", "/public/self_hosted_billing/register_email", body)
}

func (c *HTTPClient) SelfHostedVerifyEmail(ctx context.Context, code string) (*Response[SelfHostedVerifyEmailData], error) {
	body := map[string]string{"code": code}
	return makeRequest[SelfHostedVerifyEmailData](ctx, c.httpClient, c.config, "POST", "/public/self_hosted_billing/verify_email", body)
}

func (c *HTTPClient) SelfHostedStartCheckout(ctx context.Context, licenseKey string, req SelfHostedStartCheckoutRequest) (*Response[Checkout], error) {
	return makeLicenseRequest[Checkout](ctx, c.httpClient, c.config, "POST", "/public/self_hosted_billing/start_checkout", req, licenseKey)
}

func (c *HTTPClient) LicenseBillingGetPlans(ctx context.Context, licenseKey string) (*Response[[]Plan], error) {
	return makeLicenseRequest[[]Plan](ctx, c.httpClient, c.config, "GET", "/plans", nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingGetSubscription(ctx context.Context, licenseKey string) (*Response[BillingSubscription], error) {
	return makeLicenseRequest[BillingSubscription](ctx, c.httpClient, c.config, "GET", "/license_billing/subscription", nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingDeleteSubscription(ctx context.Context, licenseKey string) (*Response[interface{}], error) {
	return makeLicenseRequest[interface{}](ctx, c.httpClient, c.config, "DELETE", "/license_billing/subscription", nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingGetPaymentMethods(ctx context.Context, licenseKey string) (*Response[[]PaymentMethod], error) {
	return makeLicenseRequest[[]PaymentMethod](ctx, c.httpClient, c.config, "GET", "/license_billing/payment_methods", nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingGetSetupIntent(ctx context.Context, licenseKey string) (*Response[SetupIntent], error) {
	return makeLicenseRequest[SetupIntent](ctx, c.httpClient, c.config, "GET", "/license_billing/payment_methods/setup_intent", nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingSetDefaultPaymentMethod(ctx context.Context, licenseKey, pmID string) (*Response[interface{}], error) {
	path := fmt.Sprintf("/license_billing/payment_methods/%s/default", url.PathEscape(pmID))
	return makeLicenseRequest[interface{}](ctx, c.httpClient, c.config, "PATCH", path, nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingDeletePaymentMethod(ctx context.Context, licenseKey, pmID string) (*Response[interface{}], error) {
	path := fmt.Sprintf("/license_billing/payment_methods/%s", url.PathEscape(pmID))
	return makeLicenseRequest[interface{}](ctx, c.httpClient, c.config, "DELETE", path, nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingGetInvoices(ctx context.Context, licenseKey string) (*Response[[]Invoice], error) {
	return makeLicenseRequest[[]Invoice](ctx, c.httpClient, c.config, "GET", "/license_billing/invoices", nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingGetInvoice(ctx context.Context, licenseKey, invoiceID string) (*Response[Invoice], error) {
	path := fmt.Sprintf("/license_billing/invoices/%s", invoiceID)
	return makeLicenseRequest[Invoice](ctx, c.httpClient, c.config, "GET", path, nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingRecoverExternalID(ctx context.Context, licenseKey, newExternalID string) (*Response[LicenseRecoverExternalIDData], error) {
	body := map[string]string{"external_id": newExternalID}
	return makeLicenseRequest[LicenseRecoverExternalIDData](ctx, c.httpClient, c.config, "POST", "/license_billing/recover_external_id", body, licenseKey)
}

func (c *HTTPClient) LicenseBillingGetContext(ctx context.Context, licenseKey string) (*Response[LicenseBillingContextData], error) {
	return makeLicenseRequest[LicenseBillingContextData](ctx, c.httpClient, c.config, "GET", "/license_billing/context", nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingGetOrganisation(ctx context.Context, licenseKey string) (*Response[BillingOrganisation], error) {
	return makeLicenseRequest[BillingOrganisation](ctx, c.httpClient, c.config, "GET", "/license_billing/organisation", nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingUpdateOrganisation(ctx context.Context, licenseKey string, orgData BillingOrganisation) (*Response[BillingOrganisation], error) {
	return makeLicenseRequest[BillingOrganisation](ctx, c.httpClient, c.config, "PUT", "/license_billing/organisation", orgData, licenseKey)
}

func (c *HTTPClient) LicenseBillingUpdateOrganisationAddress(ctx context.Context, licenseKey string, addressData UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error) {
	return makeLicenseRequest[BillingOrganisation](ctx, c.httpClient, c.config, "PUT", "/license_billing/billing_address", addressData, licenseKey)
}

func (c *HTTPClient) LicenseBillingUpdateOrganisationTaxID(ctx context.Context, licenseKey string, taxData UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error) {
	return makeLicenseRequest[BillingOrganisation](ctx, c.httpClient, c.config, "PUT", "/license_billing/tax_id", taxData, licenseKey)
}

func (c *HTTPClient) LicenseBillingGetTaxIDTypes(ctx context.Context, licenseKey string) (*Response[[]TaxIDType], error) {
	return makeLicenseRequest[[]TaxIDType](ctx, c.httpClient, c.config, "GET", "/license_billing/tax_id_types", nil, licenseKey)
}

func (c *HTTPClient) LicenseBillingUpgradeSubscription(ctx context.Context, licenseKey string, req UpgradeSubscriptionRequest) (*Response[Checkout], error) {
	return makeLicenseRequest[Checkout](ctx, c.httpClient, c.config, "PUT", "/license_billing/subscription/upgrade", req, licenseKey)
}

func makeLicenseRequest[T any](ctx context.Context, httpClient *http.Client, cfg config.BillingConfiguration, method, path string, body interface{}, licenseKey string) (*Response[T], error) {
	if !strings.HasPrefix(path, "/api/v1") && !strings.HasPrefix(path, "/billing") {
		path = "/api/v1" + path
	}

	u := fmt.Sprintf("%s%s", cfg.URL, path)

	var req *http.Request
	var err error

	if body != nil {
		jsonBody, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", marshalErr)
		}
		req, err = http.NewRequestWithContext(ctx, method, u, bytes.NewReader(jsonBody))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, u, http.NoBody)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if licenseKey != "" {
		req.Header.Set("X-License-Key", licenseKey)
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))
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
		return nil, billingNonJSONError(resp, rawResp, err)
	}

	if !baseResp.Status {
		return nil, &ServiceError{StatusCode: resp.StatusCode, Message: baseResp.Message}
	}

	var data T
	if len(baseResp.Data) > 0 && string(baseResp.Data) != "null" {
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
