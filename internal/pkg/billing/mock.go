package billing

import (
	"context"
	"net/http"
	"net/http/httptest"
)

type MockBillingClient struct{}

func (m *MockBillingClient) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockBillingClient) GetUsage(ctx context.Context, orgID string) (*Response, error) {
	return &Response{
		Status:  true,
		Message: "Usage retrieved successfully",
		Data:    map[string]interface{}{"events": 100, "deliveries": 95},
	}, nil
}

func (m *MockBillingClient) GetInvoices(ctx context.Context, orgID string) (*Response, error) {
	return &Response{
		Status:  true,
		Message: "Invoices retrieved successfully",
		Data:    []map[string]interface{}{},
	}, nil
}

func (m *MockBillingClient) GetPaymentMethods(ctx context.Context, orgID string) (*Response, error) {
	return &Response{
		Status:  true,
		Message: "Payment methods retrieved successfully",
		Data:    []map[string]interface{}{},
	}, nil
}

func (m *MockBillingClient) GetSubscription(ctx context.Context, orgID string) (*Response, error) {
	return &Response{
		Status:  true,
		Message: "Subscription retrieved successfully",
		Data:    map[string]interface{}{},
	}, nil
}

func (m *MockBillingClient) GetPlans(ctx context.Context) (*Response, error) {
	return &Response{
		Status:  true,
		Message: "Plans retrieved successfully",
		Data:    []map[string]interface{}{},
	}, nil
}

func (m *MockBillingClient) GetTaxIDTypes(ctx context.Context) (*Response, error) {
	return &Response{
		Status:  true,
		Message: "Tax ID types retrieved successfully",
		Data:    []map[string]interface{}{},
	}, nil
}

func (m *MockBillingClient) CreateOrganisation(ctx context.Context, orgData interface{}) (*Response, error) {
	data, _ := orgData.(map[string]interface{})
	if data == nil || data["name"] == nil || data["name"] == "" {
		return &Response{Status: false, Message: "name is required"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Organisation created successfully",
		Data:    map[string]interface{}{"id": "org-1", "name": data["name"]},
	}, nil
}

func (m *MockBillingClient) GetOrganisation(ctx context.Context, orgID string) (*Response, error) {
	if orgID == "" {
		return &Response{Status: false, Message: "organisation ID is required"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Organisation retrieved successfully",
		Data:    map[string]interface{}{"id": orgID, "name": "Org"},
	}, nil
}

func (m *MockBillingClient) UpdateOrganisation(ctx context.Context, orgID string, orgData interface{}) (*Response, error) {
	data, _ := orgData.(map[string]interface{})
	if orgID == "" || data == nil || data["name"] == nil || data["name"] == "" {
		return &Response{Status: false, Message: "invalid organisation update"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Organisation updated successfully",
		Data:    map[string]interface{}{"id": orgID, "name": data["name"]},
	}, nil
}

func (m *MockBillingClient) UpdateOrganisationTaxID(ctx context.Context, orgID string, taxData interface{}) (*Response, error) {
	data, _ := taxData.(map[string]interface{})
	if orgID == "" || data == nil || data["tax_id_type"] == nil || data["tax_number"] == nil {
		return &Response{Status: false, Message: "invalid tax id"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Tax ID updated successfully",
		Data:    data,
	}, nil
}

func (m *MockBillingClient) UpdateOrganisationAddress(ctx context.Context, orgID string, addressData interface{}) (*Response, error) {
	data, _ := addressData.(map[string]interface{})
	if orgID == "" || data == nil || data["billing_address"] == nil {
		return &Response{Status: false, Message: "invalid address"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Address updated successfully",
		Data:    data,
	}, nil
}

func (m *MockBillingClient) GetSubscriptions(ctx context.Context, orgID string) (*Response, error) {
	return &Response{
		Status:  true,
		Message: "Subscriptions retrieved successfully",
		Data:    []map[string]interface{}{},
	}, nil
}

func (m *MockBillingClient) CreateSubscription(ctx context.Context, orgID string, subData interface{}) (*Response, error) {
	data, _ := subData.(map[string]interface{})
	if orgID == "" || data == nil || data["plan_id"] == nil || data["plan_id"] == "" {
		return &Response{Status: false, Message: "plan_id is required"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Subscription created successfully",
		Data:    map[string]interface{}{"id": "sub-1", "plan_id": data["plan_id"]},
	}, nil
}

func (m *MockBillingClient) UpdateSubscription(ctx context.Context, orgID string, subData interface{}) (*Response, error) {
	data, _ := subData.(map[string]interface{})
	if orgID == "" || data == nil || data["plan_id"] == nil || data["plan_id"] == "" {
		return &Response{Status: false, Message: "plan_id is required"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Subscription updated successfully",
		Data:    map[string]interface{}{"id": "sub-1", "plan_id": data["plan_id"]},
	}, nil
}

func (m *MockBillingClient) DeleteSubscription(ctx context.Context, orgID string) (*Response, error) {
	if orgID == "" {
		return &Response{Status: false, Message: "organisation ID is required"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Subscription deleted successfully",
		Data:    map[string]interface{}{"id": "sub-1", "status": "cancelled"},
	}, nil
}

func (m *MockBillingClient) GetSetupIntent(ctx context.Context, orgID string) (*Response, error) {
	if orgID == "" {
		return &Response{Status: false, Message: "organisation ID is required"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Setup intent retrieved successfully",
		Data:    map[string]interface{}{"client_secret": "seti_test_secret"},
	}, nil
}

func (m *MockBillingClient) CreateSetupIntent(ctx context.Context, orgID string, setupIntentData interface{}) (*Response, error) {
	if orgID == "" {
		return &Response{Status: false, Message: "organisation ID is required"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Setup intent created successfully",
		Data:    map[string]interface{}{"client_secret": "seti_test_secret"},
	}, nil
}

func (m *MockBillingClient) CreatePaymentMethod(ctx context.Context, orgID string, pmData interface{}) (*Response, error) {
	data, _ := pmData.(map[string]interface{})
	if orgID == "" || data == nil || data["payment_method_id"] == nil || data["payment_method_id"] == "" {
		return &Response{Status: false, Message: "payment_method_id is required"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Payment method created successfully",
		Data:    map[string]interface{}{"id": "pm-1"},
	}, nil
}

func (m *MockBillingClient) DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*Response, error) {
	if orgID == "" || pmID == "" {
		return &Response{Status: false, Message: "invalid payment method delete"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Payment method deleted successfully",
		Data:    map[string]interface{}{"id": pmID, "status": "deleted"},
	}, nil
}

func (m *MockBillingClient) SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*Response, error) {
	if orgID == "" || pmID == "" {
		return &Response{Status: false, Message: "invalid payment method set default"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Default payment method set successfully",
		Data:    map[string]interface{}{"id": pmID, "defaulted_at": "2025-01-01T00:00:00Z"},
	}, nil
}

func (m *MockBillingClient) GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response, error) {
	if orgID == "" || invoiceID == "" {
		return &Response{Status: false, Message: "invalid invoice request"}, nil
	}
	// Return a pdf_link that DownloadInvoice will handle
	return &Response{
		Status:  true,
		Message: "Invoice retrieved successfully",
		Data:    map[string]interface{}{"id": invoiceID, "status": "paid", "pdf_link": "http://mock-pdf-server/invoice.pdf"},
	}, nil
}

func (m *MockBillingClient) DownloadInvoice(ctx context.Context, orgID, invoiceID string) (*http.Response, error) {
	if orgID == "" || invoiceID == "" {
		return nil, &Error{Message: "invalid invoice request"}
	}

	// First get the invoice to extract pdf_link
	invoiceResp, err := m.GetInvoice(ctx, orgID, invoiceID)
	if err != nil {
		return nil, err
	}

	if !invoiceResp.Status {
		return nil, &Error{Message: invoiceResp.Message}
	}

	// Extract pdf_link
	invoiceData, ok := invoiceResp.Data.(map[string]interface{})
	if !ok {
		return nil, &Error{Message: "invalid invoice data"}
	}

	pdfLink, ok := invoiceData["pdf_link"].(string)
	if !ok || pdfLink == "" {
		return nil, &Error{Message: "invoice PDF link not found"}
	}

	// Create a test server that serves a mock PDF
	pdfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		// Write a minimal PDF content (PDF header)
		w.Write([]byte("%PDF-1.4\n1 0 obj\n<<\n/Type /Catalog\n>>\nendobj\nxref\n0 0\ntrailer\n<<\n/Root 1 0 R\n>>\n%%EOF"))
	}))
	defer pdfServer.Close()

	// Make request to the test server (ignore the pdfLink URL from GetInvoice)
	req, err := http.NewRequestWithContext(ctx, "GET", pdfServer.URL, nil)
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
