package billing

import (
	"context"
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

func (m *MockBillingClient) GetInvoice(ctx context.Context, orgID, invoiceID string) (*Response, error) {
	if orgID == "" || invoiceID == "" {
		return &Response{Status: false, Message: "invalid invoice request"}, nil
	}
	return &Response{
		Status:  true,
		Message: "Invoice retrieved successfully",
		Data:    map[string]interface{}{"id": invoiceID, "status": "paid"},
	}, nil
}

func (m *MockBillingClient) DownloadInvoice(ctx context.Context, orgID, invoiceID string) ([]byte, error) {
	return []byte("fake pdf content"), nil
}
