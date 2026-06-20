package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/portal_links"
	"github.com/frain-dev/convoy/internal/users"
)

type BillingIntegrationTestSuite struct {
	suite.Suite
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultUser     *datastore.User
}

type countingBillingClient struct {
	*billing.MockBillingClient
	completeCalls int
	lastStart     billing.StartGuestCheckoutRequest
	startErr      error
}

func (c *countingBillingClient) CompleteGuestCheckout(ctx context.Context, req billing.CompleteGuestCheckoutRequest) (*billing.Response[billing.GuestCheckoutCompletion], error) {
	c.completeCalls++
	return c.MockBillingClient.CompleteGuestCheckout(ctx, req)
}

func (c *countingBillingClient) StartGuestCheckout(ctx context.Context, req billing.StartGuestCheckoutRequest) (*billing.Response[billing.Checkout], error) {
	c.lastStart = req
	if c.startErr != nil {
		return nil, c.startErr
	}
	return c.MockBillingClient.StartGuestCheckout(ctx, req)
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
	// The suite clones one DB in SetupSuite and shares it across tests, so purge before
	// each test to stop seeded roles (e.g. instance admins) leaking between cases.
	testdb.PurgeDB(s.T(), s.ConvoyApp.A.DB)

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

	userRepo := users.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	apiRepo := api_keys.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	portalLinkRepo := portal_links.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *BillingIntegrationTestSuite) TearDownTest() {
	metrics.Reset()
}

func (s *BillingIntegrationTestSuite) Test_GetBillingConfigIncludesStrategy() {
	req := createRequest(http.MethodGet, "/ui/billing/config", "", nil)
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

	require.Equal(s.T(), "Billing configuration retrieved", response["message"])
	require.True(s.T(), response["status"].(bool))

	data := response["data"].(map[string]interface{})
	require.Equal(s.T(), "cloud", data["strategy"])
}

func (s *BillingIntegrationTestSuite) Test_GetBillingConfigHidesActiveCheckoutForNonInstanceAdmin() {
	restore := s.seedActiveSelfHostedCheckout()
	defer restore()

	req := createRequest(http.MethodGet, "/ui/billing/config", "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	data := response["data"].(map[string]interface{})
	selfHosted := data["self_hosted"].(map[string]interface{})
	require.NotContains(s.T(), selfHosted, "active_checkout")
	require.NotContains(s.T(), selfHosted, "active_checkout_attempt_id")
	require.NotContains(s.T(), selfHosted, "checkout_id")
	require.NotContains(s.T(), selfHosted, "external_id")
}

func (s *BillingIntegrationTestSuite) Test_GetBillingConfigHidesActiveCheckoutForSelfHostedOrganisationAdmin() {
	originalCfg := s.ConvoyApp.A.Cfg
	cfg := s.ConvoyApp.A.Cfg
	cfg.Billing.APIKey = ""
	s.ConvoyApp.A.Cfg = cfg
	s.ConvoyApp.cfg = cfg
	defer func() {
		s.ConvoyApp.A.Cfg = originalCfg
		s.ConvoyApp.cfg = originalCfg
	}()

	restore := s.seedActiveSelfHostedCheckout()
	defer restore()
	_, err := testdb.SeedDefaultOrganisationWithRole(s.ConvoyApp.A.DB, s.DefaultUser, auth.RoleOrganisationAdmin)
	require.NoError(s.T(), err)

	req := createRequest(http.MethodGet, "/ui/billing/config", "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	data := response["data"].(map[string]interface{})
	selfHosted := data["self_hosted"].(map[string]interface{})
	require.NotContains(s.T(), selfHosted, "active_checkout")
	require.NotContains(s.T(), selfHosted, "active_checkout_attempt_id")
	require.NotContains(s.T(), selfHosted, "checkout_id")
	require.NotContains(s.T(), selfHosted, "external_id")
}

func (s *BillingIntegrationTestSuite) Test_GetBillingConfigIncludesActiveCheckoutForInstanceAdmin() {
	restore := s.seedActiveSelfHostedCheckout()
	defer restore()
	_, err := testdb.SeedDefaultOrganisationWithRole(s.ConvoyApp.A.DB, s.DefaultUser, auth.RoleInstanceAdmin)
	require.NoError(s.T(), err)

	req := createRequest(http.MethodGet, "/ui/billing/config", "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	data := response["data"].(map[string]interface{})
	selfHosted := data["self_hosted"].(map[string]interface{})
	activeCheckout := selfHosted["active_checkout"].(map[string]interface{})
	require.Equal(s.T(), "attempt-active", activeCheckout["attempt_id"])
	require.Equal(s.T(), "checkout-active", activeCheckout["checkout_id"])
	require.Equal(s.T(), "https://checkout.example.test/session", activeCheckout["checkout_url"])
	require.Equal(s.T(), "checkout-active", selfHosted["checkout_id"])
	require.Equal(s.T(), "external-active", selfHosted["external_id"])
}

func (s *BillingIntegrationTestSuite) Test_SelfHostedOrganisationAdminCanStartCheckout() {
	originalCfg := s.ConvoyApp.A.Cfg
	originalRouter := s.Router
	originalClient := s.ConvoyApp.A.BillingClient
	cfgSvc := configuration.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	savedBillingCfg, err := cfgSvc.LoadInstanceBillingConfig(context.Background())
	if err != nil {
		savedBillingCfg, err = testdb.SeedConfiguration(s.ConvoyApp.A.DB)
	}
	require.NoError(s.T(), err)

	_, err = testdb.SeedDefaultOrganisationWithRole(s.ConvoyApp.A.DB, s.DefaultUser, auth.RoleOrganisationAdmin)
	require.NoError(s.T(), err)
	cfg := s.ConvoyApp.A.Cfg
	cfg.Billing.APIKey = ""
	s.ConvoyApp.A.Cfg = cfg
	s.ConvoyApp.cfg = cfg
	s.ConvoyApp.A.BillingClient = &billing.MockBillingClient{}
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
	defer func() {
		_ = cfgSvc.UpdateInstanceBillingConfig(context.Background(), savedBillingCfg)
		s.ConvoyApp.A.Cfg = originalCfg
		s.ConvoyApp.cfg = originalCfg
		s.ConvoyApp.A.BillingClient = originalClient
		s.Router = originalRouter
	}()

	attemptID := s.startSelfHostedCheckout("org-admin@example.com")

	require.NotEmpty(s.T(), attemptID)
}

func (s *BillingIntegrationTestSuite) Test_StartSelfHostedCheckout_SendsCheckoutLicenseKeyForResubscribe() {
	originalCfg := s.ConvoyApp.A.Cfg
	originalRouter := s.Router
	originalClient := s.ConvoyApp.A.BillingClient
	cfgSvc := configuration.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	savedBillingCfg, err := cfgSvc.LoadInstanceBillingConfig(context.Background())
	if err != nil {
		savedBillingCfg, err = testdb.SeedConfiguration(s.ConvoyApp.A.DB)
	}
	require.NoError(s.T(), err)

	_, err = testdb.SeedDefaultOrganisationWithRole(s.ConvoyApp.A.DB, s.DefaultUser, auth.RoleInstanceAdmin)
	require.NoError(s.T(), err)

	// Persist a purchased guest key so the handler resubscribes against it.
	billingCfg, err := cfgSvc.LoadInstanceBillingConfig(context.Background())
	require.NoError(s.T(), err)
	billingCfg.CheckoutLicenseKey = "RESUB-KEY-123"
	require.NoError(s.T(), cfgSvc.UpdateInstanceBillingConfig(context.Background(), billingCfg))

	client := &countingBillingClient{MockBillingClient: &billing.MockBillingClient{}}
	cfg := s.ConvoyApp.A.Cfg
	cfg.Billing.APIKey = ""
	s.ConvoyApp.A.Cfg = cfg
	s.ConvoyApp.cfg = cfg
	s.ConvoyApp.A.BillingClient = client
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
	defer func() {
		_ = cfgSvc.UpdateInstanceBillingConfig(context.Background(), savedBillingCfg)
		s.ConvoyApp.A.Cfg = originalCfg
		s.ConvoyApp.cfg = originalCfg
		s.ConvoyApp.A.BillingClient = originalClient
		s.Router = originalRouter
	}()

	attemptID := s.startSelfHostedCheckout("resub@example.com")
	require.NotEmpty(s.T(), attemptID)
	require.Equal(s.T(), "RESUB-KEY-123", client.lastStart.LicenseKey)
}

func (s *BillingIntegrationTestSuite) Test_StartSelfHostedCheckout_ResubscribeWithoutEmail() {
	originalCfg := s.ConvoyApp.A.Cfg
	originalRouter := s.Router
	originalClient := s.ConvoyApp.A.BillingClient
	cfgSvc := configuration.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	savedBillingCfg, err := cfgSvc.LoadInstanceBillingConfig(context.Background())
	if err != nil {
		savedBillingCfg, err = testdb.SeedConfiguration(s.ConvoyApp.A.DB)
	}
	require.NoError(s.T(), err)

	_, err = testdb.SeedDefaultOrganisationWithRole(s.ConvoyApp.A.DB, s.DefaultUser, auth.RoleInstanceAdmin)
	require.NoError(s.T(), err)

	// A purchased guest key makes this a resubscribe, so email may be omitted.
	billingCfg, err := cfgSvc.LoadInstanceBillingConfig(context.Background())
	require.NoError(s.T(), err)
	billingCfg.CheckoutLicenseKey = "RESUB-KEY-456"
	require.NoError(s.T(), cfgSvc.UpdateInstanceBillingConfig(context.Background(), billingCfg))

	client := &countingBillingClient{MockBillingClient: &billing.MockBillingClient{}}
	cfg := s.ConvoyApp.A.Cfg
	cfg.Billing.APIKey = ""
	s.ConvoyApp.A.Cfg = cfg
	s.ConvoyApp.cfg = cfg
	s.ConvoyApp.A.BillingClient = client
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
	defer func() {
		_ = cfgSvc.UpdateInstanceBillingConfig(context.Background(), savedBillingCfg)
		s.ConvoyApp.A.Cfg = originalCfg
		s.ConvoyApp.cfg = originalCfg
		s.ConvoyApp.A.BillingClient = originalClient
		s.Router = originalRouter
	}()

	body, err := json.Marshal(map[string]string{
		"plan_id": "self_hosted_premium",
		"host":    "https://customer.example.com",
	})
	require.NoError(s.T(), err)
	req := createRequest(http.MethodPost, "/ui/billing/sh_checkout/start", "", bytes.NewBuffer(body))
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusOK, w.Code, w.Body.String())
	require.Equal(s.T(), "RESUB-KEY-456", client.lastStart.LicenseKey)
	require.Empty(s.T(), client.lastStart.Email)
}

func (s *BillingIntegrationTestSuite) Test_StartSelfHostedCheckout_SurfacesResubscribeBlock() {
	originalCfg := s.ConvoyApp.A.Cfg
	originalRouter := s.Router
	originalClient := s.ConvoyApp.A.BillingClient
	cfgSvc := configuration.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	savedBillingCfg, err := cfgSvc.LoadInstanceBillingConfig(context.Background())
	if err != nil {
		savedBillingCfg, err = testdb.SeedConfiguration(s.ConvoyApp.A.DB)
	}
	require.NoError(s.T(), err)

	_, err = testdb.SeedDefaultOrganisationWithRole(s.ConvoyApp.A.DB, s.DefaultUser, auth.RoleInstanceAdmin)
	require.NoError(s.T(), err)

	// Overwatch returns 409 for a duplicate checkout; the handler must surface it.
	client := &countingBillingClient{
		MockBillingClient: &billing.MockBillingClient{},
		startErr:          &billing.Error{StatusCode: http.StatusConflict, Message: "an active subscription already exists; cancel it before resubscribing"},
	}
	cfg := s.ConvoyApp.A.Cfg
	cfg.Billing.APIKey = ""
	s.ConvoyApp.A.Cfg = cfg
	s.ConvoyApp.cfg = cfg
	s.ConvoyApp.A.BillingClient = client
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
	defer func() {
		_ = cfgSvc.UpdateInstanceBillingConfig(context.Background(), savedBillingCfg)
		s.ConvoyApp.A.Cfg = originalCfg
		s.ConvoyApp.cfg = originalCfg
		s.ConvoyApp.A.BillingClient = originalClient
		s.Router = originalRouter
	}()

	body, err := json.Marshal(map[string]string{
		"email":   "resub-blocked@example.com",
		"plan_id": "self_hosted_premium",
		"host":    "https://customer.example.com",
	})
	require.NoError(s.T(), err)
	req := createRequest(http.MethodPost, "/ui/billing/sh_checkout/start", "", bytes.NewBuffer(body))
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusConflict, w.Code, w.Body.String())
	var response map[string]interface{}
	require.NoError(s.T(), json.Unmarshal(w.Body.Bytes(), &response))
	require.Contains(s.T(), response["message"], "cancel it before resubscribing")
}

func (s *BillingIntegrationTestSuite) seedActiveSelfHostedCheckout() func() {
	cfgSvc := configuration.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	cfg, err := cfgSvc.LoadInstanceBillingConfig(context.Background())
	if err != nil {
		cfg, err = testdb.SeedConfiguration(s.ConvoyApp.A.DB)
	}
	require.NoError(s.T(), err)
	previousCfg := *cfg
	if cfg.CheckoutAttempts != nil {
		previousCfg.CheckoutAttempts = make(map[string]datastore.SelfHostedCheckoutAttempt, len(cfg.CheckoutAttempts))
		for id, attempt := range cfg.CheckoutAttempts {
			previousCfg.CheckoutAttempts[id] = attempt
		}
	}

	now := time.Now()
	cfg.ActiveCheckoutAttemptID = "attempt-active"
	cfg.CheckoutID = "checkout-active"
	cfg.ExternalID = "external-active"
	cfg.CheckoutAttempts = map[string]datastore.SelfHostedCheckoutAttempt{
		"attempt-active": {
			AttemptID:     "attempt-active",
			CheckoutID:    "checkout-active",
			CheckoutURL:   "https://checkout.example.test/session",
			CheckoutNonce: "nonce",
			PlanID:        "plan-premium",
			Interval:      "yearly",
			Status:        "pending",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}

	err = cfgSvc.UpdateInstanceBillingConfig(context.Background(), cfg)
	require.NoError(s.T(), err)

	return func() {
		err := cfgSvc.UpdateInstanceBillingConfig(context.Background(), &previousCfg)
		require.NoError(s.T(), err)
	}
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

func (s *BillingIntegrationTestSuite) Test_SupersededSelfHostedCheckoutDoesNotCallBillingCompletion() {
	originalCfg := s.ConvoyApp.A.Cfg
	originalRouter := s.Router
	originalClient := s.ConvoyApp.A.BillingClient
	cfgSvc := configuration.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	savedBillingCfg, err := cfgSvc.LoadInstanceBillingConfig(context.Background())
	if err != nil {
		savedBillingCfg, err = testdb.SeedConfiguration(s.ConvoyApp.A.DB)
	}
	require.NoError(s.T(), err)

	client := &countingBillingClient{MockBillingClient: &billing.MockBillingClient{}}
	_, err = testdb.SeedDefaultOrganisationWithRole(s.ConvoyApp.A.DB, s.DefaultUser, auth.RoleInstanceAdmin)
	require.NoError(s.T(), err)
	cfg := s.ConvoyApp.A.Cfg
	cfg.Billing.APIKey = ""
	s.ConvoyApp.A.Cfg = cfg
	s.ConvoyApp.cfg = cfg
	s.ConvoyApp.A.BillingClient = client
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
	defer func() {
		_ = cfgSvc.UpdateInstanceBillingConfig(context.Background(), savedBillingCfg)
		s.ConvoyApp.A.Cfg = originalCfg
		s.ConvoyApp.cfg = originalCfg
		s.ConvoyApp.A.BillingClient = originalClient
		s.Router = originalRouter
	}()

	firstAttemptID := s.startSelfHostedCheckout("buyer-one@example.com")
	_ = s.startSelfHostedCheckout("buyer-two@example.com")

	body, err := json.Marshal(map[string]string{"attempt_id": firstAttemptID})
	require.NoError(s.T(), err)
	req := createRequest(http.MethodPost, "/ui/billing/sh_checkout/complete", "", bytes.NewBuffer(body))
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusNotFound, w.Code)
	require.Equal(s.T(), 0, client.completeCalls)
}

func (s *BillingIntegrationTestSuite) startSelfHostedCheckout(email string) string {
	body, err := json.Marshal(map[string]string{
		"email":   email,
		"plan_id": "self_hosted_premium",
		"host":    "https://customer.example.com",
	})
	require.NoError(s.T(), err)
	req := createRequest(http.MethodPost, "/ui/billing/sh_checkout/start", "", bytes.NewBuffer(body))
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusOK, w.Code, w.Body.String())
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)
	data := response["data"].(map[string]interface{})
	return data["attempt_id"].(string)
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

func (s *BillingIntegrationTestSuite) Test_GetInternalOrganisationID() {
	url := fmt.Sprintf("/ui/billing/organisations/%s/internal_id", s.DefaultOrg.UID)
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

	require.Equal(s.T(), "Internal organisation ID retrieved successfully", response["message"])
	require.True(s.T(), response["status"].(bool))
	data, ok := response["data"].(map[string]interface{})
	require.True(s.T(), ok)
	require.Contains(s.T(), data, "id")
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

func (s *BillingIntegrationTestSuite) Test_OnboardSubscription() {
	onboardData := map[string]interface{}{
		"plan_id": "plan-uuid-123",
		"host":    "https://app.getconvoy.io",
	}

	body, _ := json.Marshal(onboardData)
	url := fmt.Sprintf("/ui/billing/organisations/%s/subscriptions/onboard", s.DefaultOrg.UID)
	req := createRequest(http.MethodPost, url, "", bytes.NewBuffer(body))
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Checkout session created successfully", response["message"])
	require.True(s.T(), response["status"].(bool))

	// Verify checkout_url is in response
	data, ok := response["data"].(map[string]interface{})
	require.True(s.T(), ok)
	require.Contains(s.T(), data, "checkout_url")
}

func (s *BillingIntegrationTestSuite) Test_UpgradeSubscription() {
	upgradeData := map[string]interface{}{
		"plan_id": "plan-uuid-456",
		"host":    "https://app.getconvoy.io",
	}

	body, _ := json.Marshal(upgradeData)
	url := fmt.Sprintf("/ui/billing/organisations/%s/subscriptions/sub-123/upgrade", s.DefaultOrg.UID)
	req := createRequest(http.MethodPut, url, "", bytes.NewBuffer(body))
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "Checkout session created successfully", response["message"])
	require.True(s.T(), response["status"].(bool))

	// Verify checkout_url is in response
	data, ok := response["data"].(map[string]interface{})
	require.True(s.T(), ok)
	require.Contains(s.T(), data, "checkout_url")
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

func (s *BillingIntegrationTestSuite) Test_DownloadInvoice() {
	url := fmt.Sprintf("/ui/billing/organisations/%s/invoices/inv-1/download", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		s.T().Logf("Response body: %s", w.Body.String())
	}
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Verify response headers
	require.Equal(s.T(), "application/pdf", w.Header().Get("Content-Type"))
	require.Contains(s.T(), w.Header().Get("Content-Disposition"), "attachment")
	require.Contains(s.T(), w.Header().Get("Content-Disposition"), "invoice-inv-1.pdf")

	// Verify response body contains PDF content
	require.Greater(s.T(), len(w.Body.Bytes()), 0)
	require.Contains(s.T(), string(w.Body.Bytes()[:10]), "%PDF")
}

func (s *BillingIntegrationTestSuite) Test_DownloadInvoice_NotFound() {
	// Test with empty invoice ID to trigger validation error
	url := fmt.Sprintf("/ui/billing/organisations/%s/invoices//download", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	// Should return a bad request status for missing invoice ID
	require.Equal(s.T(), http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(s.T(), err)
	require.False(s.T(), response["status"].(bool))
	require.Contains(s.T(), response["message"].(string), "invoice ID")
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
