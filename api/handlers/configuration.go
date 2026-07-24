package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

func (h *Handler) GetConfiguration(w http.ResponseWriter, r *http.Request) {
	configuration, err := h.A.ConfigRepo.LoadConfiguration(r.Context())
	if err != nil && !errors.Is(err, datastore.ErrConfigNotFound) {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	var configResponse []*models.ConfigurationResponse
	if configuration != nil {
		redactConfigurationSecrets(configuration)

		c := &models.ConfigurationResponse{
			Configuration: configuration,
			ApiVersion:    convoy.GetVersion(),
		}

		configResponse = append(configResponse, c)
	}

	_ = render.Render(w, r, util.NewServerResponse("Configuration fetched successfully", configResponse, http.StatusOK))
}

// redactConfigurationSecrets strips commercial-license, checkout, and storage
// secrets from the instance configuration before it leaves GetConfiguration. That
// handler is served on the UI and portal-api routes to any authenticated user or
// portal-link holder, none of which need these fields (the config UI reads only
// analytics/signup/retention/storage metadata). Billing management surfaces read
// license/checkout fields via their own gated endpoints, so redacting here removes
// a license-key, checkout-nonce, and blob-storage credential leak without breaking
// any current caller.
func redactConfigurationSecrets(c *datastore.Configuration) {
	if c == nil {
		return
	}

	c.LicenseKey = ""
	c.CheckoutLicenseKey = ""
	c.LicenseKeySource = ""
	c.CheckoutAttempts = nil
	c.ActiveCheckoutAttemptID = ""
	c.CheckoutID = ""
	c.ExternalID = ""

	redactStoragePolicySecrets(c)
}

// redactStoragePolicySecrets clears blob-storage credentials from the storage
// policy, keeping only the non-sensitive location metadata the config UI renders.
// Stripping is keyed on struct presence rather than StoragePolicy.Type so a
// misconfigured Type cannot leak a populated credential set.
func redactStoragePolicySecrets(c *datastore.Configuration) {
	if c.StoragePolicy == nil {
		return
	}

	if s3 := c.StoragePolicy.S3; s3 != nil {
		c.StoragePolicy.S3 = &datastore.S3Storage{
			Bucket:   s3.Bucket,
			Endpoint: s3.Endpoint,
			Region:   s3.Region,
		}
	}

	if azure := c.StoragePolicy.AzureBlob; azure != nil {
		c.StoragePolicy.AzureBlob = &datastore.AzureBlobStorage{
			AccountName:   azure.AccountName,
			ContainerName: azure.ContainerName,
			Endpoint:      azure.Endpoint,
			Prefix:        azure.Prefix,
		}
	}

	if c.StoragePolicy.OnPrem != nil {
		// The on-prem path points at the host filesystem backing instance
		// backups; it is not needed by any config reader and must not leak.
		c.StoragePolicy.OnPrem = &datastore.OnPremStorage{}
	}
}

func (h *Handler) CreateConfiguration(w http.ResponseWriter, r *http.Request) {
	var newConfig models.Configuration
	if err := util.ReadJSON(r, &newConfig); err != nil {
		h.A.Logger.Errorf("Failed to parse configuration request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	if err := newConfig.Validate(); err != nil {
		h.A.Logger.Errorf("Configuration validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid configuration provided", http.StatusBadRequest))
		return
	}

	cc := services.CreateConfigService{
		ConfigRepo: h.A.ConfigRepo,
		NewConfig:  &newConfig,
	}

	configuration, err := cc.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	c := &models.ConfigurationResponse{
		Configuration: configuration,
		ApiVersion:    convoy.GetVersion(),
	}

	_ = render.Render(w, r, util.NewServerResponse("Configuration created successfully", c, http.StatusCreated))
}

func (h *Handler) UpdateConfiguration(w http.ResponseWriter, r *http.Request) {
	var newConfig models.Configuration
	if err := util.ReadJSON(r, &newConfig); err != nil {
		h.A.Logger.Errorf("Failed to parse configuration update request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	if err := newConfig.Validate(); err != nil {
		h.A.Logger.Errorf("Configuration update validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid configuration provided", http.StatusBadRequest))
		return
	}

	uc := services.UpdateConfigService{
		ConfigRepo: h.A.ConfigRepo,
		Config:     &newConfig,
		Logger:     h.A.Logger,
	}

	configuration, err := uc.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	c := &models.ConfigurationResponse{
		Configuration: configuration,
		ApiVersion:    convoy.GetVersion(),
	}

	_ = render.Render(w, r, util.NewServerResponse("Configuration updated successfully", c, http.StatusAccepted))
}

func (h *Handler) GetAuthConfiguration(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Get()
	if err != nil {
		h.A.Logger.ErrorContext(r.Context(), "failed to load configuration", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("failed to load configuration", http.StatusBadRequest))
		return
	}
	useOrgBilling := cfg.UsesOrgBilling() && h.A.BillingClient != nil
	// Floor with the in-memory env/file license so a DB read failure or an absent
	// config row does not misreport an env-licensed instance as OSS on this
	// pre-login auth surface. A non-empty persisted key (env resolved at boot, or a
	// guest purchase) takes precedence. Unlike the billing management endpoints,
	// this stays available on DB errors rather than failing closed.
	instanceLicenseKey := strings.TrimSpace(cfg.LicenseKey)
	if instanceBilling, err := configuration.New(h.A.Logger, h.A.DB).LoadInstanceBillingConfig(r.Context()); err == nil && instanceBilling != nil {
		if k := strings.TrimSpace(instanceBilling.LicenseKey); k != "" {
			instanceLicenseKey = k
		}
	}
	slug := strings.TrimSpace(r.URL.Query().Get("slug"))

	ssoEnabled := h.A.Licenser.EnterpriseSSO()
	if useOrgBilling && slug != "" {
		result, err := services.LookupWorkspaceBySlug(r.Context(), slug, services.ResolveWorkspaceBySlugDeps{
			BillingClient: h.A.BillingClient,
			OrgRepo:       h.A.OrgRepo,
			Logger:        h.A.Logger,
			Cfg:           cfg,
		})
		if err != nil {
			if errors.Is(err, services.ErrWorkspaceNotFound) {
				_ = render.Render(w, r, util.NewErrorResponse("workspace not found", http.StatusBadRequest))
				return
			}
			h.A.Logger.ErrorContext(r.Context(), "failed to resolve workspace slug", "error", err, "slug", slug)
			_ = render.Render(w, r, util.NewErrorResponse("failed to resolve workspace", http.StatusServiceUnavailable))
			return
		}
		ssoEnabled = result.SSOAvailable
	}

	authConfig := map[string]interface{}{
		"billing_strategy":  string(cfg.BillingMode(instanceLicenseKey)),
		"is_signup_enabled": cfg.Auth.IsSignupEnabled,
		"google_oauth": map[string]interface{}{
			"enabled":      cfg.Auth.GoogleOAuth.Enabled && h.A.Licenser.GoogleOAuth(),
			"client_id":    cfg.Auth.GoogleOAuth.ClientID,
			"redirect_url": cfg.Auth.GoogleOAuth.RedirectURL,
		},
		"sso": map[string]interface{}{
			"enabled":      ssoEnabled,
			"redirect_url": cfg.Auth.SSO.RedirectURL,
		},
	}

	_ = render.Render(w, r, util.NewServerResponse("Auth configuration fetched successfully", authConfig, http.StatusOK))
}
