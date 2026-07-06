package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

const (
	// Default cache TTL for entitlements
	defaultCacheTTL = 10 * time.Minute
	// Community plan limits
	communityProjectLimit = 2
	communityOrgLimit     = 1
	communityUserLimit    = 1

	// Community mode resolves its enabled-project set from the database so the
	// API and worker processes agree on which projects are active. A frozen
	// startup snapshot is per-process, so a project created after a process
	// started (the worker, typically) would never be enabled there and its
	// event deliveries would be gated. communityProjectCacheTTL is the steady
	// refresh interval; communityProjectMissTTL bounds extra refreshes when a
	// delivery asks about a project not yet in the cached set.
	communityProjectCacheTTL = 1 * time.Minute
	communityProjectMissTTL  = 5 * time.Second

	// License statuses that are definitively negative. A license in one of these
	// states must not serve entitlements (fail closed on the read gates).
	licenseStatusSuspended = "suspended"
	licenseStatusRevoked   = "revoked"
	licenseStatusExpired   = "expired"
	licenseStatusNotFound  = "not_found"

	licenseStatusTrialExpired = "trial_expired"
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

	// refreshCancel stops the background refresh goroutine; refreshDone is
	// closed when that goroutine returns. Both are nil for community and
	// billing-only licensers, which have no background refresh.
	refreshCancel context.CancelFunc
	refreshDone   chan struct{}

	orgRepo     datastore.OrganisationRepository
	userRepo    datastore.UserRepository
	projectRepo datastore.ProjectRepository

	// For community mode: track enabled projects
	mu                    sync.RWMutex
	enabledProjects       map[string]bool
	projectsFetchedAt     time.Time
	lastProjectMutationAt time.Time
	// isCommunity is atomic because a self-hosted trial can flip it from false to
	// true at runtime when the trial expires (degradeToCommunity), while request
	// goroutines read it locklessly on the feature/limit gates. A licensed
	// licenser starts false; a community licenser (empty key, or a degraded
	// expired trial) is true.
	isCommunity atomic.Bool
	denyLimits  bool

	logger log.Logger
}

// Config holds configuration for the license service licenser
type LicenserConfig struct {
	LicenseKey    string
	UseOrgBilling bool
	Client        *Client
	OrgRepo       datastore.OrganisationRepository
	UserRepo      datastore.UserRepository
	ProjectRepo   datastore.ProjectRepository
	CacheTTL      time.Duration
	Logger        log.Logger
}

// NewLicenser creates a new license service licenser
func NewLicenser(cfg LicenserConfig) (*Licenser, error) {
	if util.IsStringEmpty(cfg.LicenseKey) {
		if cfg.UseOrgBilling {
			return newBillingOnlyLicenser(cfg)
		}
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
		// Boot policy per validation outcome:
		//   - Expired self-hosted TRIAL: do NOT fail startup. validateAndCache has
		//     already degraded this licenser to the community/OSS floor in place
		//     (isCommunity set, community projects loaded), so return it as a
		//     community licenser and skip the background refresh (a community
		//     licenser has none). This is the "lapsed trial boots to free tier,
		//     not bricked" requirement.
		//   - Any other error (transport, suspended, revoked, paid-expired): keep
		//     the existing loud failure. A paid key that cannot validate must not
		//     silently boot.
		if errors.Is(err, ErrLicenseTrialExpired) {
			return licenser, nil
		}
		return nil, fmt.Errorf("failed to validate license: %w", err)
	}

	// The synchronous feature gates only read cached state, so a live process
	// would keep serving premium entitlements after a suspension until it
	// restarted. The background refresh re-validates on the cache TTL so a
	// suspension or revocation takes effect within one TTL without a restart.
	licenser.startBackgroundRefresh()

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

	l := &Licenser{
		enabledProjects:   enabledProjects,
		projectsFetchedAt: time.Now(),
		orgRepo:           cfg.OrgRepo,
		userRepo:          cfg.UserRepo,
		projectRepo:       cfg.ProjectRepo,
		entitlements:      make(map[string]EntitlementValue),
		logger:            cfg.Logger,
	}
	l.isCommunity.Store(true)
	return l, nil
}

func newBillingOnlyLicenser(cfg LicenserConfig) (*Licenser, error) {
	return &Licenser{
		denyLimits:   true,
		orgRepo:      cfg.OrgRepo,
		userRepo:     cfg.UserRepo,
		projectRepo:  cfg.ProjectRepo,
		entitlements: make(map[string]EntitlementValue),
		logger:       cfg.Logger,
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

// validateAndCache validates the license and caches entitlements.
//
// Failure policy (entitlements follow money):
//   - Definitive-negative result (suspended, revoked, expired, not found): fail
//     closed. Record the status, drop the cached entitlements, and advance
//     lastFetch so the authoritative "no access" is cached for the TTL. The read
//     gates then stop serving premium features on this live process.
//   - Transient / transport error (network failure, non-200, unmarshal error):
//     fail open. Keep the last-good entitlements and do NOT advance lastFetch, so
//     a network blip cannot revoke a paying customer's access.
//   - Expired self-hosted TRIAL (ErrLicenseTrialExpired): degrade to the
//     community/OSS floor in place (isCommunity, community project set), NOT fail
//     closed. This is deliberately distinct from the definitive-negative statuses
//     below so an expired trial lands on the free tier instead of denying access.
func (l *Licenser) validateAndCache(ctx context.Context) error {
	data, err := l.client.ValidateLicense(ctx, l.licenseKey)
	if err != nil {
		if errors.Is(err, ErrLicenseTrialExpired) {
			l.degradeToCommunity(ctx)
			return err
		}
		if status, ok := definitiveNegativeStatus(err); ok {
			l.entitlementsMu.Lock()
			l.status = status
			l.entitlements = make(map[string]EntitlementValue)
			l.lastFetch = time.Now()
			l.entitlementsMu.Unlock()
		}
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

func (l *Licenser) degradeToCommunity(ctx context.Context) {
	if l.isCommunity.Load() {
		return
	}

	enabled, err := enforceProjectLimit(ctx, l.projectRepo)
	if err != nil {
		if l.logger != nil {
			l.logger.Warnf("trial expired but failed to load community projects, will retry on next refresh: %v", err)
		}
		return
	}

	l.mu.Lock()
	l.enabledProjects = enabled
	l.projectsFetchedAt = time.Now()
	l.mu.Unlock()

	l.entitlementsMu.Lock()
	l.entitlements = make(map[string]EntitlementValue)
	l.status = licenseStatusTrialExpired
	l.lastFetch = time.Now()
	l.entitlementsMu.Unlock()

	l.isCommunity.Store(true)

	if l.logger != nil {
		l.logger.Warn("self-hosted trial expired; instance degraded to community/OSS floor")
	}
}

// ensureValidCache ensures entitlements are fresh (within TTL)
func (l *Licenser) ensureValidCache(ctx context.Context) error {
	if l.isCommunity.Load() || l.denyLimits {
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
	if l.isCommunity.Load() || l.denyLimits {
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

// definitiveNegativeStatus maps a client validation error to the license status
// it represents, reporting whether the error is a definitive-negative result
// (as opposed to a transient/transport error). The typed errors returned by the
// client for these statuses are compared with errors.Is.
func definitiveNegativeStatus(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrLicenseSuspended):
		return licenseStatusSuspended, true
	case errors.Is(err, ErrLicenseRevoked):
		return licenseStatusRevoked, true
	case errors.Is(err, ErrLicenseExpired):
		return licenseStatusExpired, true
	case errors.Is(err, ErrLicenseNotFound):
		return licenseStatusNotFound, true
	default:
		return "", false
	}
}

// isLicenseUsable reports whether the last authoritative validation left the
// license in a usable state. A suspended or revoked license fails closed on
// every read gate, mirroring validateAndCache which drops entitlements for the
// same statuses. Guarded by entitlementsMu; callers must not already hold it.
func (l *Licenser) isLicenseUsable() bool {
	l.entitlementsMu.RLock()
	defer l.entitlementsMu.RUnlock()
	return l.status != licenseStatusSuspended && l.status != licenseStatusRevoked
}

// getEntitlement retrieves an entitlement value
func (l *Licenser) getEntitlement(key string) EntitlementValue {
	if l.isCommunity.Load() || l.denyLimits {
		return false
	}

	if !l.isLicenseUsable() {
		return false
	}

	l.entitlementsMu.RLock()
	defer l.entitlementsMu.RUnlock()

	return l.entitlements[key]
}

// hasFeature checks if a feature is enabled
func (l *Licenser) hasFeature(key string) bool {
	if l.isCommunity.Load() || l.denyLimits {
		return false
	}

	if !l.isLicenseUsable() {
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
	if l.denyLimits {
		return false, nil
	}
	if l.isCommunity.Load() {
		count, err := countFunc(ctx)
		if err != nil {
			return false, err
		}
		return count < communityLimit, nil
	}

	if err := l.ensureValidCache(ctx); err != nil {
		return false, err
	}

	// Fail closed: a suspended or revoked license grants no limit headroom.
	if !l.isLicenseUsable() {
		return false, nil
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
	if l.isCommunity.Load() || l.denyLimits {
		return false, nil
	}

	if err := l.ensureValidCache(ctx); err != nil {
		return false, err
	}

	// Fail closed: a suspended or revoked license is not multi-user.
	if !l.isLicenseUsable() {
		return false, nil
	}

	l.entitlementsMu.RLock()
	limit, exists := GetNumberEntitlement(l.entitlements, "user_limit")
	l.entitlementsMu.RUnlock()
	if !exists {
		return false, nil
	}

	return limit == -1 || limit > 1, nil
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
	if l.isCommunity.Load() {
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

func (l *Licenser) BasicAuthEndpointAuth() bool {
	return l.hasFeature("basic_auth_endpoint_auth")
}

func (l *Licenser) EndpointURLTemplates() bool {
	return l.hasFeature("endpoint_url_templates")
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
	if l.isCommunity.Load() {
		orgLimit, orgLimitExists = communityOrgLimit, true
		userLimit, userLimitExists = communityUserLimit, true
		projectLimit, projectLimitExists = communityProjectLimit, true
	}

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
	featureList["BasicAuthEndpointAuth"] = l.BasicAuthEndpointAuth()
	featureList["EndpointURLTemplates"] = l.EndpointURLTemplates()
	featureList["UseForwardProxy"] = l.UseForwardProxy()
	featureList["AsynqMonitoring"] = l.AsynqMonitoring()
	featureList["AgentExecutionMode"] = l.AgentExecutionMode()

	return json.Marshal(featureList)
}

func (l *Licenser) ProjectEnabled(projectID string) bool {
	if !l.isCommunity.Load() {
		return true
	}

	l.refreshEnabledProjectsIfStale(projectID)

	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.enabledProjects[projectID]
}

// refreshEnabledProjectsIfStale reconciles the community enabled-project set
// with the database. It refreshes when the cached set is older than the TTL, or
// when projectID is unknown (likely created after this process started) and we
// have not refreshed within the shorter miss window. This keeps the delivery
// hot path cheap while still letting the worker pick up newly created projects.
//
// Failure policy: if the database read fails we keep the last-known set and do
// not change any project's enablement. This fails closed (a not-yet-enabled
// project stays gated until a successful refresh) rather than bypassing the
// community project limit on a transient DB error.
func (l *Licenser) refreshEnabledProjectsIfStale(projectID string) {
	l.mu.RLock()
	_, present := l.enabledProjects[projectID]
	age := time.Since(l.projectsFetchedAt)
	l.mu.RUnlock()

	if age < communityProjectCacheTTL && (present || age < communityProjectMissTTL) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// startedAt is captured before the read so we can detect an AddEnabledProject
	// or RemoveEnabledProject that committed while the read was in flight on this
	// (API) process. The DB read may predate that mutation, so applying its
	// result would clobber the fresher local state (re-enable a deleted project
	// or drop a newly created one). When that happens we discard this refresh and
	// let the next one reconcile.
	startedAt := time.Now()

	enabled, err := enforceProjectLimit(ctx, l.projectRepo)
	if err != nil {
		if l.logger != nil {
			l.logger.Warnf("failed to refresh community enabled projects, keeping cached set: %v", err)
		}
		// Mark the attempt so a failing DB does not trigger a refresh on every
		// delivery; the miss/TTL windows still apply on the next call.
		l.mu.Lock()
		l.projectsFetchedAt = time.Now()
		l.mu.Unlock()
		return
	}

	l.mu.Lock()
	if !l.lastProjectMutationAt.After(startedAt) {
		l.enabledProjects = enabled
	}
	l.projectsFetchedAt = time.Now()
	l.mu.Unlock()
}

func (l *Licenser) AddEnabledProject(projectID string) {
	if !l.isCommunity.Load() {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.enabledProjects) >= communityProjectLimit {
		return
	}

	l.enabledProjects[projectID] = true
	// Record the mutation so an in-flight DB refresh that predates this create
	// does not overwrite it with a staler set.
	l.lastProjectMutationAt = time.Now()
}

func (l *Licenser) RemoveEnabledProject(projectID string) {
	if !l.isCommunity.Load() {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.enabledProjects, projectID)
	// Record the mutation so an in-flight DB refresh that predates this delete
	// does not re-enable the removed project.
	l.lastProjectMutationAt = time.Now()
}

// startBackgroundRefresh launches a ticker that re-validates the license every
// cacheTTL. Each tick runs validateAndCache, which applies the fail-closed /
// fail-open policy documented there. This is what makes a live process reflect a
// suspension or revocation without a restart: the read gates read the cached
// status and entitlements the ticker maintains.
func (l *Licenser) startBackgroundRefresh() {
	ctx, cancel := context.WithCancel(context.Background())
	l.refreshCancel = cancel
	l.refreshDone = make(chan struct{})

	go func() {
		defer close(l.refreshDone)

		ticker := time.NewTicker(l.cacheTTL)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rctx, rcancel := context.WithTimeout(ctx, 10*time.Second)
				if err := l.validateAndCache(rctx); err != nil && l.logger != nil {
					l.logger.Warnf("background license refresh failed: %v", err)
				}
				rcancel()
			}
		}
	}()
}

// Close stops the background refresh goroutine and waits for it to return. It is
// safe to call on any licenser (community and billing-only have no goroutine) and
// more than once (cancel is idempotent and the done channel stays closed).
func (l *Licenser) Close() {
	if l.refreshCancel == nil {
		return
	}
	l.refreshCancel()
	<-l.refreshDone
}

var ErrLicenseExpired = errors.New("license expired")

// ErrLicenseTrialExpired is the sentinel for an expired self-hosted trial. It is
// deliberately separate from ErrLicenseExpired (paid): the client maps the
// distinct billing-service "Trial has expired" message to it, and the licenser routes
// it to community-floor degradation instead of the paid fail-closed/grace paths.
var ErrLicenseTrialExpired = errors.New("trial expired")
