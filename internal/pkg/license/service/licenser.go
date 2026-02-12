package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

const (
	// Default cache TTL for entitlements
	defaultCacheTTL = 10 * time.Minute
	// Community plan limits
	communityProjectLimit = 2
	communityOrgLimit     = 1
	communityUserLimit    = 1
)

// Licenser implements the license.Licenser interface using the license service
type Licenser struct {
	client         *Client
	licenseKey     string
	entitlements   map[string]EntitlementValue
	entitlementsMu sync.RWMutex
	lastFetch      time.Time
	cacheTTL       time.Duration
	expiresAt      *time.Time
	status         string

	orgRepo     datastore.OrganisationRepository
	userRepo    datastore.UserRepository
	projectRepo datastore.ProjectRepository

	// For community mode: track enabled projects
	mu              sync.RWMutex
	enabledProjects map[string]bool
	isCommunity     bool

	logger log.StdLogger
}

// Config holds configuration for the license service licenser
type LicenserConfig struct {
	LicenseKey  string
	Client      *Client
	OrgRepo     datastore.OrganisationRepository
	UserRepo    datastore.UserRepository
	ProjectRepo datastore.ProjectRepository
	CacheTTL    time.Duration
	Logger      log.StdLogger
}

// NewLicenser creates a new license service licenser
func NewLicenser(cfg LicenserConfig) (*Licenser, error) {
	if util.IsStringEmpty(cfg.LicenseKey) {
		// No license key provided, return community licenser
		return newCommunityLicenser(cfg)
	}

	// Create license service client if not provided (with hardcoded defaults)
	if cfg.Client == nil {
		cfg.Client = NewClient(Config{
			Host:         DefaultOverwatchHost,
			ValidatePath: DefaultValidatePath,
			Timeout:      DefaultTimeout,
			RetryCount:   DefaultRetryCount,
			Logger:       cfg.Logger,
		})
	}

	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = defaultCacheTTL
	}

	licenser := &Licenser{
		client:       cfg.Client,
		licenseKey:   cfg.LicenseKey,
		orgRepo:      cfg.OrgRepo,
		userRepo:     cfg.UserRepo,
		projectRepo:  cfg.ProjectRepo,
		cacheTTL:     cfg.CacheTTL,
		entitlements: make(map[string]EntitlementValue),
		logger:       cfg.Logger,
	}

	// Initial validation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := licenser.validateAndCache(ctx); err != nil {
		return nil, fmt.Errorf("failed to validate license: %w", err)
	}

	return licenser, nil
}

// newCommunityLicenser creates a community licenser with limited features
func newCommunityLicenser(cfg LicenserConfig) (*Licenser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	enabledProjects, err := enforceProjectLimit(ctx, cfg.ProjectRepo)
	if err != nil {
		return nil, err
	}

	return &Licenser{
		isCommunity:     true,
		enabledProjects: enabledProjects,
		orgRepo:         cfg.OrgRepo,
		userRepo:        cfg.UserRepo,
		projectRepo:     cfg.ProjectRepo,
		entitlements:    make(map[string]EntitlementValue),
		logger:          cfg.Logger,
	}, nil
}

// enforceProjectLimit enforces project limit for community mode
func enforceProjectLimit(ctx context.Context, projectRepo datastore.ProjectRepository) (map[string]bool, error) {
	projects, err := projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	if len(projects) > communityProjectLimit {
		// Enabled projects exceed limit, allow only the last projects to be active
		projects = projects[len(projects)-communityProjectLimit:]
	}

	m := map[string]bool{}
	for _, p := range projects {
		m[p.UID] = true
	}

	return m, nil
}

// validateAndCache validates the license and caches entitlements
func (l *Licenser) validateAndCache(ctx context.Context) error {
	data, err := l.client.ValidateLicense(ctx, l.licenseKey)
	if err != nil {
		return err
	}

	l.entitlementsMu.Lock()
	defer l.entitlementsMu.Unlock()

	entitlementsMap, err := data.GetEntitlementsMap()
	if err != nil {
		return fmt.Errorf("failed to parse entitlements: %w", err)
	}

	l.entitlements = ParseEntitlements(entitlementsMap)
	l.lastFetch = time.Now()
	l.expiresAt = data.ExpiresAt
	l.status = data.Status

	return nil
}

// ensureValidCache ensures entitlements are fresh (within TTL)
func (l *Licenser) ensureValidCache(ctx context.Context) error {
	if l.isCommunity {
		return nil
	}

	l.entitlementsMu.RLock()
	needsRefresh := time.Since(l.lastFetch) > l.cacheTTL
	l.entitlementsMu.RUnlock()

	if needsRefresh {
		return l.validateAndCache(ctx)
	}

	return nil
}

// checkExpiry checks if the license has expired
func (l *Licenser) checkExpiry() error {
	if l.isCommunity {
		return nil
	}

	l.entitlementsMu.RLock()
	expiresAt := l.expiresAt
	l.entitlementsMu.RUnlock()

	if expiresAt == nil {
		// Cloud licenses have no expiry
		return nil
	}

	now := time.Now()
	if now.After(*expiresAt) {
		v := now.Sub(*expiresAt)
		const days = 21 * 24 * time.Hour // 21 days grace period

		if v < days {
			daysAgo := int64(v.Hours() / 24)
			if l.logger != nil {
				l.logger.Warnf("license expired %d days ago, access to features will be revoked in %d days", daysAgo, 21-daysAgo)
			}
			return nil
		}

		return ErrLicenseExpired
	}

	return nil
}

// getEntitlement retrieves an entitlement value
func (l *Licenser) getEntitlement(key string) EntitlementValue {
	if l.isCommunity {
		return false
	}

	l.entitlementsMu.RLock()
	defer l.entitlementsMu.RUnlock()

	return l.entitlements[key]
}

// hasFeature checks if a feature is enabled
func (l *Licenser) hasFeature(key string) bool {
	if l.isCommunity {
		return false
	}

	if err := l.checkExpiry(); err != nil {
		return false
	}

	l.entitlementsMu.RLock()
	defer l.entitlementsMu.RUnlock()
	return GetBoolEntitlement(l.entitlements, key)
}

// checkLimit checks if a count is within the allowed limit
func (l *Licenser) checkLimit(ctx context.Context, countFunc func(context.Context) (int64, error), limitKey string, communityLimit int64) (bool, error) {
	if l.isCommunity {
		count, err := countFunc(ctx)
		if err != nil {
			return false, err
		}
		return count < communityLimit, nil
	}

	if err := l.ensureValidCache(ctx); err != nil {
		return false, err
	}

	if err := l.checkExpiry(); err != nil {
		return false, err
	}

	count, err := countFunc(ctx)
	if err != nil {
		return false, err
	}

	l.entitlementsMu.RLock()
	limit, exists := GetNumberEntitlement(l.entitlements, limitKey)
	l.entitlementsMu.RUnlock()
	if !exists {
		return false, nil
	}

	if limit == -1 {
		return true, nil
	}

	return count < limit, nil
}

// CheckOrgLimit checks if org creation is allowed based on org_limit entitlement
func (l *Licenser) CheckOrgLimit(ctx context.Context) (bool, error) {
	return l.checkLimit(ctx, func(ctx context.Context) (int64, error) {
		return l.orgRepo.CountOrganisations(ctx)
	}, "org_limit", communityOrgLimit)
}

// CheckUserLimit checks if user creation is allowed based on user_limit entitlement
func (l *Licenser) CheckUserLimit(ctx context.Context) (bool, error) {
	return l.checkLimit(ctx, func(ctx context.Context) (int64, error) {
		return l.userRepo.CountUsers(ctx)
	}, "user_limit", communityUserLimit)
}

// CheckProjectLimit checks if project creation is allowed based on project_limit entitlement
func (l *Licenser) CheckProjectLimit(ctx context.Context) (bool, error) {
	return l.checkLimit(ctx, func(ctx context.Context) (int64, error) {
		return l.projectRepo.CountProjects(ctx)
	}, "project_limit", communityProjectLimit)
}

func (l *Licenser) IsMultiUserMode(ctx context.Context) (bool, error) {
	if l.isCommunity {
		return false, nil
	}

	if err := l.ensureValidCache(ctx); err != nil {
		return false, err
	}

	l.entitlementsMu.RLock()
	limit, exists := GetNumberEntitlement(l.entitlements, "user_limit")
	l.entitlementsMu.RUnlock()
	if !exists {
		return false, nil
	}

	return limit > 1, nil
}

// Implement license.Licenser interface methods

func (l *Licenser) CreateOrg(ctx context.Context) (bool, error) {
	return l.checkLimit(ctx, func(ctx context.Context) (int64, error) {
		return l.orgRepo.CountOrganisations(ctx)
	}, "org_limit", communityOrgLimit)
}

func (l *Licenser) CreateUser(ctx context.Context) (bool, error) {
	return l.checkLimit(ctx, func(ctx context.Context) (int64, error) {
		return l.userRepo.CountUsers(ctx)
	}, "user_limit", communityUserLimit)
}

func (l *Licenser) CreateProject(ctx context.Context) (bool, error) {
	return l.checkLimit(ctx, func(ctx context.Context) (int64, error) {
		return l.projectRepo.CountProjects(ctx)
	}, "project_limit", communityProjectLimit)
}

func (l *Licenser) UseForwardProxy() bool {
	return l.hasFeature("use_forward_proxy")
}

func (l *Licenser) CanExportPrometheusMetrics() bool {
	return l.hasFeature("export_prometheus_metrics")
}

func (l *Licenser) AdvancedEndpointMgmt() bool {
	return l.hasFeature("advanced_endpoint_mgmt")
}

func (l *Licenser) AdvancedSubscriptions() bool {
	return l.hasFeature("advanced_subscriptions")
}

func (l *Licenser) Transformations() bool {
	return l.hasFeature("webhook_transformations")
}

func (l *Licenser) AsynqMonitoring() bool {
	return l.hasFeature("asynq_monitoring")
}

func (l *Licenser) PortalLinks() bool {
	return l.hasFeature("portal_links")
}

func (l *Licenser) ConsumerPoolTuning() bool {
	return l.hasFeature("consumer_pool_tuning")
}

func (l *Licenser) AdvancedWebhookFiltering() bool {
	return l.hasFeature("advanced_webhook_filtering")
}

func (l *Licenser) CircuitBreaking() bool {
	return l.hasFeature("circuit_breaking")
}

func (l *Licenser) IngestRate() bool {
	if l.isCommunity {
		return false
	}
	return l.hasFeature("ingest_rate_limit")
}

func (l *Licenser) AgentExecutionMode() bool {
	return l.hasFeature("agent_execution_mode")
}

func (l *Licenser) IpRules() bool {
	return l.hasFeature("ip_rules")
}

func (l *Licenser) EnterpriseSSO() bool {
	return l.hasFeature("enterprise_sso")
}

func (l *Licenser) GoogleOAuth() bool {
	return l.hasFeature("google_oauth")
}

func (l *Licenser) DatadogTracing() bool {
	return l.hasFeature("datadog_tracing")
}

func (l *Licenser) ReadReplica() bool {
	return l.hasFeature("read_replica")
}

func (l *Licenser) CredentialEncryption() bool {
	return l.hasFeature("credential_encryption")
}

func (l *Licenser) CustomCertificateAuthority() bool {
	return l.hasFeature("custom_certificate_authority")
}

func (l *Licenser) StaticIP() bool {
	return l.hasFeature("static_ip")
}

func (l *Licenser) RetentionPolicy() bool {
	return l.hasFeature("webhook_archiving")
}

func (l *Licenser) WebhookAnalytics() bool {
	return l.hasFeature("webhook_analytics")
}

func (l *Licenser) MutualTLS() bool {
	return l.hasFeature("mutual_tls")
}

func (l *Licenser) OAuth2EndpointAuth() bool {
	return l.hasFeature("oauth2_endpoint_auth")
}

func (l *Licenser) FeatureListJSON(ctx context.Context) (json.RawMessage, error) {
	if err := l.ensureValidCache(ctx); err != nil {
		return nil, err
	}

	// Build feature list with dynamic limits
	featureList := make(map[string]interface{})

	// Check dynamic limits using new methods with enriched information
	orgAllowed, err := l.CheckOrgLimit(ctx)
	if err != nil {
		return nil, err
	}

	userAllowed, err := l.CheckUserLimit(ctx)
	if err != nil {
		return nil, err
	}

	projectAllowed, err := l.CheckProjectLimit(ctx)
	if err != nil {
		return nil, err
	}

	// Acquire read lock for all entitlement accesses
	l.entitlementsMu.RLock()
	orgLimit, orgLimitExists := GetNumberEntitlement(l.entitlements, "org_limit")
	userLimit, userLimitExists := GetNumberEntitlement(l.entitlements, "user_limit")
	projectLimit, projectLimitExists := GetNumberEntitlement(l.entitlements, "project_limit")
	l.entitlementsMu.RUnlock()

	orgCount, err := l.orgRepo.CountOrganisations(ctx)
	if err != nil {
		return nil, err
	}
	orgAvailable := orgLimitExists && (orgLimit > 0 || orgLimit == -1)
	orgLimitReached := orgAvailable && !orgAllowed
	featureList["org_limit"] = map[string]interface{}{
		"limit":         orgLimit,
		"allowed":       orgAllowed,
		"current":       orgCount,
		"available":     orgAvailable,
		"limit_reached": orgLimitReached,
	}

	userCount, err := l.userRepo.CountUsers(ctx)
	if err != nil {
		return nil, err
	}
	userAvailable := userLimitExists && (userLimit > 0 || userLimit == -1)
	userLimitReached := userAvailable && !userAllowed
	featureList["user_limit"] = map[string]interface{}{
		"limit":         userLimit,
		"allowed":       userAllowed,
		"current":       userCount,
		"available":     userAvailable,
		"limit_reached": userLimitReached,
	}

	projectCount, err := l.projectRepo.CountProjects(ctx)
	if err != nil {
		return nil, err
	}
	projectAvailable := projectLimitExists && (projectLimit > 0 || projectLimit == -1)
	projectLimitReached := projectAvailable && !projectAllowed
	featureList["project_limit"] = map[string]interface{}{
		"limit":         projectLimit,
		"allowed":       projectAllowed,
		"current":       projectCount,
		"available":     projectAvailable,
		"limit_reached": projectLimitReached,
	}

	// Add boolean features (removed deprecated ones)
	featureList["EnterpriseSSO"] = l.EnterpriseSSO()
	featureList["PortalLinks"] = l.PortalLinks()
	featureList["Transformations"] = l.Transformations()
	featureList["AdvancedSubscriptions"] = l.AdvancedSubscriptions()
	featureList["WebhookAnalytics"] = l.WebhookAnalytics()
	featureList["AdvancedWebhookFiltering"] = l.AdvancedWebhookFiltering()
	featureList["AdvancedEndpointMgmt"] = l.AdvancedEndpointMgmt()
	featureList["CircuitBreaking"] = l.CircuitBreaking()
	featureList["ConsumerPoolTuning"] = l.ConsumerPoolTuning()
	featureList["GoogleOAuth"] = l.GoogleOAuth()
	featureList["CanExportPrometheusMetrics"] = l.CanExportPrometheusMetrics()
	featureList["ReadReplica"] = l.ReadReplica()
	featureList["CredentialEncryption"] = l.CredentialEncryption()
	featureList["IpRules"] = l.IpRules()
	featureList["RetentionPolicy"] = l.RetentionPolicy()
	featureList["MutualTLS"] = l.MutualTLS()
	featureList["DatadogTracing"] = l.DatadogTracing()
	featureList["CustomCertificateAuthority"] = l.CustomCertificateAuthority()
	featureList["StaticIP"] = l.StaticIP()
	featureList["OAuth2EndpointAuth"] = l.OAuth2EndpointAuth()
	featureList["UseForwardProxy"] = l.UseForwardProxy()
	featureList["AsynqMonitoring"] = l.AsynqMonitoring()
	featureList["AgentExecutionMode"] = l.AgentExecutionMode()

	return json.Marshal(featureList)
}

func (l *Licenser) ProjectEnabled(projectID string) bool {
	if l.isCommunity {
		l.mu.RLock()
		defer l.mu.RUnlock()
		return l.enabledProjects[projectID]
	}
	return true
}

func (l *Licenser) AddEnabledProject(projectID string) {
	if !l.isCommunity {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.enabledProjects) >= communityProjectLimit {
		return
	}

	l.enabledProjects[projectID] = true
}

func (l *Licenser) RemoveEnabledProject(projectID string) {
	if !l.isCommunity {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.enabledProjects, projectID)
}

var ErrLicenseExpired = errors.New("license expired")
