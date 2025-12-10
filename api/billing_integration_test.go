package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
)

type BillingIntegrationTestSuite struct {
	suite.Suite
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultUser     *datastore.User
}

func (s *BillingIntegrationTestSuite) SetupSuite() {
	s.ConvoyApp = buildServer(s.T())

	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm-billing.json")
	require.NoError(s.T(), err)

	cfg, err := config.Get()
	require.NoError(s.T(), err)
	s.ConvoyApp.A.Cfg = cfg
	s.ConvoyApp.cfg = cfg

	mockClient := &billing.MockBillingClient{}
	s.ConvoyApp.A.BillingClient = mockClient

	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *BillingIntegrationTestSuite) SetupTest() {
	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisationWithRole(s.ConvoyApp.A.DB, user, auth.RoleBillingAdmin)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *BillingIntegrationTestSuite) TearDownTest() {
	metrics.Reset()
}

func (s *BillingIntegrationTestSuite) Test_GetBillingEnabled() {
	req := createRequest(http.MethodGet, "/ui/billing/enabled", "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Billing status retrieved", response["message"])
	require.True(s.T(), response["status"].(bool))

	data := response["data"].(map[string]interface{})
	require.True(s.T(), data["enabled"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_GetPlans() {
	req := createRequest(http.MethodGet, "/ui/billing/plans", "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Plans retrieved successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_GetTaxIDTypes() {
	req := createRequest(http.MethodGet, "/ui/billing/tax_id_types", "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Tax ID types retrieved successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_GetInvoices() {
	url := fmt.Sprintf("/ui/billing/organisations/%s/invoices", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Invoices retrieved successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_GetSubscription() {
	url := fmt.Sprintf("/ui/billing/organisations/%s/subscription", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Subscription retrieved successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_GetPaymentMethods() {
	url := fmt.Sprintf("/ui/billing/organisations/%s/payment_methods", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Payment methods retrieved successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_CreateOrganisation() {
	orgData := map[string]interface{}{
		"name":          "Test Org",
		"billing_email": "test@example.com",
	}

	body, _ := json.Marshal(orgData)
	req := createRequest(http.MethodPost, "/ui/billing/organisations", "", bytes.NewBuffer(body))
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusCreated, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Organisation created successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_GetOrganisation() {
	url := fmt.Sprintf("/ui/billing/organisations/%s", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Organisation retrieved successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_UpdateOrganisation() {
	orgData := map[string]interface{}{
		"name": "Updated Org",
	}

	body, _ := json.Marshal(orgData)
	url := fmt.Sprintf("/ui/billing/organisations/%s", s.DefaultOrg.UID)
	req := createRequest(http.MethodPut, url, "", bytes.NewBuffer(body))
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Organisation updated successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_CreateSubscription() {
	subData := map[string]interface{}{
		"plan_id": "plan-1",
	}

	body, _ := json.Marshal(subData)
	url := fmt.Sprintf("/ui/billing/organisations/%s/subscriptions", s.DefaultOrg.UID)
	req := createRequest(http.MethodPost, url, "", bytes.NewBuffer(body))
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusCreated, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Subscription created successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_GetInvoice() {
	url := fmt.Sprintf("/ui/billing/organisations/%s/invoices/inv-1", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Invoice retrieved successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_GetSetupIntent() {
	url := fmt.Sprintf("/ui/billing/organisations/%s/payment_methods/setup_intent", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Setup intent retrieved successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_GetSubscriptions() {
	url := fmt.Sprintf("/ui/billing/organisations/%s/subscriptions", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Subscriptions retrieved successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_UpdateOrganisationTaxID() {
	taxData := map[string]interface{}{
		"tax_id_type": "ein",
		"tax_number":  "12-3456789",
	}

	body, _ := json.Marshal(taxData)
	url := fmt.Sprintf("/ui/billing/organisations/%s/tax_id", s.DefaultOrg.UID)
	req := createRequest(http.MethodPut, url, "", bytes.NewBuffer(body))
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Tax ID updated successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func (s *BillingIntegrationTestSuite) Test_UpdateOrganisationAddress() {
	addressData := map[string]interface{}{
		"billing_address": "123 Main St",
		"billing_city":    "New York",
		"billing_state":   "NY",
		"billing_zip":     "10001",
		"billing_country": "US",
	}

	body, _ := json.Marshal(addressData)
	url := fmt.Sprintf("/ui/billing/organisations/%s/address", s.DefaultOrg.UID)
	req := createRequest(http.MethodPut, url, "", bytes.NewBuffer(body))
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Address updated successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
}

func TestBillingIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(BillingIntegrationTestSuite))
}
