package billing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
)

type MockBillingClient struct {
	mu            sync.RWMutex
	organisations map[string]BillingOrganisation
	// CreateOrganisationLicenseKey, when set, is returned as Data.LicenseKey from CreateOrganisation (for tests).
	CreateOrganisationLicenseKey string
	// GetOrganisationLicenseKey, when set, is returned as Data.Key from GetOrganisationLicense (for tests).
	GetOrganisationLicenseKey string
}

func (m *MockBillingClient) ensureOrganisation(orgID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.organisations == nil {
		m.organisations = make(map[string]BillingOrganisation)
	}
	if _, exists := m.organisations[orgID]; !exists {
		m.organisations[orgID] = BillingOrganisation{
			ID:         orgID,
			ExternalID: orgID,
			Name:       "Test Org",
		}
	}
}

func (m *MockBillingClient) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockBillingClient) GetUsage(ctx context.Context, orgID string) (*Response[Usage], error) {
	return &Response[Usage]{
		Status:  true,
		Message: "Usage retrieved successfully",
		Data: Usage{
			OrganisationID: orgID,
			Received:       UsageMetrics{Volume: 100, Bytes: 1000},
			Sent:           UsageMetrics{Volume: 95, Bytes: 950},
		},
	}, nil
}

func (m *MockBillingClient) GetInvoices(ctx context.Context, orgID string) (*Response[[]Invoice], error) {
	m.ensureOrganisation(orgID)

	return &Response[[]Invoice]{
		Status:  true,
		Message: "Invoices retrieved successfully",
		Data:    []Invoice{},
	}, nil
}

func (m *MockBillingClient) GetPaymentMethods(ctx context.Context, orgID string) (*Response[[]PaymentMethod], error) {
	m.ensureOrganisation(orgID)

	return &Response[[]PaymentMethod]{
		Status:  true,
		Message: "Payment methods retrieved successfully",
		Data:    []PaymentMethod{},
	}, nil
}

func (m *MockBillingClient) GetSubscription(ctx context.Context, orgID string) (*Response[BillingSubscription], error) {
	m.ensureOrganisation(orgID)

	return &Response[BillingSubscription]{
		Status:  true,
		Message: "Subscription retrieved successfully",
		Data:    BillingSubscription{},
	}, nil
}

func (m *MockBillingClient) GetPlans(ctx context.Context) (*Response[[]Plan], error) {
	return &Response[[]Plan]{
		Status:  true,
		Message: "Plans retrieved successfully",
		Data:    []Plan{},
	}, nil
}

func (m *MockBillingClient) GetTaxIDTypes(ctx context.Context) (*Response[[]TaxIDType], error) {
	return &Response[[]TaxIDType]{
		Status:  true,
		Message: "Tax ID types retrieved successfully",
		Data:    []TaxIDType{},
	}, nil
}

func (m *MockBillingClient) CreateOrganisation(ctx context.Context, orgData BillingOrganisation) (*Response[BillingOrganisation], error) {
	if orgData.Name == "" {
		return nil, &Error{Message: "name is required"}
	}

	m.mu.Lock()
	if m.organisations == nil {
		m.organisations = make(map[string]BillingOrganisation)
	}
	createdOrg := BillingOrganisation{
		ID:           orgData.ExternalID,
		Name:         orgData.Name,
		ExternalID:   orgData.ExternalID,
		BillingEmail: orgData.BillingEmail,
		Host:         orgData.Host,
	}
	if m.CreateOrganisationLicenseKey != "" {
		createdOrg.LicenseKey = m.CreateOrganisationLicenseKey
	}
	m.organisations[orgData.ExternalID] = createdOrg
	m.mu.Unlock()

	return &Response[BillingOrganisation]{
		Status:  true,
		Message: "Organisation created successfully",
		Data:    createdOrg,
	}, nil
}

func (m *MockBillingClient) GetOrganisation(ctx context.Context, orgID string) (*Response[BillingOrganisation], error) {
	if orgID == "" {
		return nil, &Error{Message: "organisation ID is required"}
	}

	m.ensureOrganisation(orgID)

	m.mu.RLock()
	org := m.organisations[orgID]
	m.mu.RUnlock()

	return &Response[BillingOrganisation]{
		Status:  true,
		Message: "Organisation retrieved successfully",
		Data:    org,
	}, nil
}

func (m *MockBillingClient) GetOrganisationLicense(ctx context.Context, orgID string) (*Response[OrganisationLicense], error) {
	m.ensureOrganisation(orgID)
	data := OrganisationLicense{
		Organisation: &BillingOrganisation{
			ExternalID: orgID,
			LicenseKey: m.GetOrganisationLicenseKey,
		},
	}
	return &Response[OrganisationLicense]{
		Status:  true,
		Message: "OK",
		Data:    data,
	}, nil
}

func (m *MockBillingClient) GetWorkspaceConfigBySlug(ctx context.Context, slug string) (*Response[WorkspaceConfigData], error) {
	if slug == "" {
		return nil, &Error{Message: "slug is required"}
	}
	return &Response[WorkspaceConfigData]{
		Status:  true,
		Message: "OK",
		Data:    WorkspaceConfigData{ExternalID: slug, SSOAvailable: false},
	}, nil
}

func (m *MockBillingClient) UpdateOrganisation(ctx context.Context, orgID string, orgData BillingOrganisation) (*Response[BillingOrganisation], error) {
	if orgID == "" || orgData.Name == "" {
		return nil, &Error{Message: "invalid organisation update"}
	}

	m.ensureOrganisation(orgID)

	m.mu.Lock()
	org := m.organisations[orgID]
	org.Name = orgData.Name
	if orgData.BillingEmail != "" {
		org.BillingEmail = orgData.BillingEmail
	}
	m.organisations[orgID] = org
	m.mu.Unlock()

	return &Response[BillingOrganisation]{
		Status:  true,
		Message: "Organisation updated successfully",
		Data:    org,
	}, nil
}

func (m *MockBillingClient) UpdateOrganisationTaxID(ctx context.Context, orgID string, taxData UpdateOrganisationTaxIDRequest) (*Response[BillingOrganisation], error) {
	if orgID == "" || taxData.TaxIDType == "" || taxData.TaxNumber == "" {
		return nil, &Error{Message: "invalid tax id"}
	}

	m.ensureOrganisation(orgID)

	m.mu.RLock()
	org := m.organisations[orgID]
	m.mu.RUnlock()

	return &Response[BillingOrganisation]{
		Status:  true,
		Message: "Tax ID updated successfully",
		Data:    org,
	}, nil
}

func (m *MockBillingClient) UpdateOrganisationAddress(ctx context.Context, orgID string, addressData UpdateOrganisationAddressRequest) (*Response[BillingOrganisation], error) {
	if orgID == "" {
		return nil, &Error{Message: "invalid address"}
	}

	m.ensureOrganisation(orgID)

	m.mu.RLock()
	org := m.organisations[orgID]
	m.mu.RUnlock()

	return &Response[BillingOrganisation]{
		Status:  true,
		Message: "Address updated successfully",
		Data:    org,
	}, nil
}

func (m *MockBillingClient) GetSubscriptions(ctx context.Context, orgID string) (*Response[[]BillingSubscription], error) {
	m.ensureOrganisation(orgID)

	return &Response[[]BillingSubscription]{
		Status:  true,
		Message: "Subscriptions retrieved successfully",
		Data:    []BillingSubscription{},
	}, nil
}

func (m *MockBillingClient) OnboardSubscription(ctx context.Context, orgID string, req OnboardSubscriptionRequest) (*Response[Checkout], error) {
	if orgID == "" || req.PlanID == "" || req.Host == "" {
		return nil, &Error{Message: "organisation ID, plan ID, and host are required"}
	}

	m.ensureOrganisation(orgID)

	return &Response[Checkout]{
		Status:  true,
		Message: "Checkout session created successfully",
		Data:    Checkout{CheckoutURL: "https://checkout.maple.com/mock-checkout"},
	}, nil
}

func (m *MockBillingClient) UpgradeSubscription(ctx context.Context, orgID, subscriptionID string, req UpgradeSubscriptionRequest) (*Response[Checkout], error) {
	if orgID == "" || subscriptionID == "" || req.PlanID == "" || req.Host == "" {
		return nil, &Error{Message: "organisation ID, subscription ID, plan ID, and host are required"}
	}

	m.ensureOrganisation(orgID)

	return &Response[Checkout]{
		Status:  true,
		Message: "Checkout session created successfully",
		Data:    Checkout{CheckoutURL: "https://checkout.maple.com/mock-checkout"},
	}, nil
}

func (m *MockBillingClient) DeleteSubscription(ctx context.Context, orgID, subscriptionID string) (*Response[interface{}], error) {
	if orgID == "" || subscriptionID == "" {
		return nil, &Error{Message: "organisation ID and subscription ID are required"}
	}

	m.ensureOrganisation(orgID)

	return &Response[interface{}]{
		Status:  true,
		Message: "Subscription deleted successfully",
		Data:    nil,
	}, nil
}

func (m *MockBillingClient) GetSetupIntent(ctx context.Context, orgID string) (*Response[SetupIntent], error) {
	if orgID == "" {
		return nil, &Error{Message: "organisation ID is required"}
	}

	m.ensureOrganisation(orgID)

	return &Response[SetupIntent]{
		Status:  true,
		Message: "Setup intent retrieved successfully",
		Data:    SetupIntent{IntentSecret: "seti_test_secret"},
	}, nil
}

func (m *MockBillingClient) CreateSetupIntent(ctx context.Context, orgID string, setupIntentData CreateSetupIntentRequest) (*Response[SetupIntent], error) {
	if orgID == "" {
		return nil, &Error{Message: "organisation ID is required"}
	}

	m.ensureOrganisation(orgID)

	return &Response[SetupIntent]{
		Status:  true,
		Message: "Setup intent created successfully",
		Data:    SetupIntent{IntentSecret: "seti_test_secret"},
	}, nil
}

func (m *MockBillingClient) DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	if orgID == "" || pmID == "" {
		return nil, &Error{Message: "invalid payment method delete"}
	}

	m.ensureOrganisation(orgID)

	return &Response[interface{}]{
		Status:  true,
		Message: "Payment method deleted successfully",
		Data:    nil,
	}, nil
}

func (m *MockBillingClient) SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*Response[interface{}], error) {
	if orgID == "" || pmID == "" {
		return nil, &Error{Message: "invalid payment method set default"}
	}

	m.ensureOrganisation(orgID)

	return &Response[interface{}]{
		Status:  true,
		Message: "Default payment method set successfully",
		Data:    nil,
	}, nil
}

func (m *MockBillingClient) GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response[Invoice], error) {
	if orgID == "" || invoiceID == "" {
		return nil, &Error{Message: "invalid invoice request"}
	}

	m.ensureOrganisation(orgID)

	return &Response[Invoice]{
		Status:  true,
		Message: "Invoice retrieved successfully",
		Data:    Invoice{ID: invoiceID, Status: "paid", PDFLink: "http://mock-pdf-server/invoice.pdf"},
	}, nil
}

func (m *MockBillingClient) DownloadInvoice(ctx context.Context, orgID, invoiceID string) (*http.Response, error) {
	if orgID == "" || invoiceID == "" {
		return nil, &Error{Message: "invalid invoice request"}
	}

	invoiceResp, err := m.GetInvoice(ctx, orgID, invoiceID)
	if err != nil {
		return nil, err
	}

	if !invoiceResp.Status {
		return nil, &Error{Message: invoiceResp.Message}
	}

	pdfLink := invoiceResp.Data.PDFLink
	if pdfLink == "" {
		return nil, &Error{Message: "invoice PDF link not found"}
	}

	pdfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("%PDF-1.4\n1 0 obj\n<<\n/Type /Catalog\n>>\nendobj\nxref\n0 0\ntrailer\n<<\n/Root 1 0 R\n>>\n%%EOF"))
	}))
	defer pdfServer.Close()

	req, err := http.NewRequestWithContext(ctx, "GET", pdfServer.URL, http.NoBody)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}
