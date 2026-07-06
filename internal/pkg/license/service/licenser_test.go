package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestCommunityFeatureListExposesBuiltInLimits(t *testing.T) {
	ctx := context.Background()
	licenser, err := NewLicenser(LicenserConfig{
		OrgRepo:     communityOrgRepo{count: communityOrgLimit},
		UserRepo:    communityUserRepo{count: communityUserLimit},
		ProjectRepo: communityProjectRepo{count: communityProjectLimit},
	})
	require.NoError(t, err)

	raw, err := licenser.FeatureListJSON(ctx)
	require.NoError(t, err)

	var features map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &features))

	requireLimit(t, features["org_limit"], communityOrgLimit, communityOrgLimit, false, true, true)
	requireLimit(t, features["user_limit"], communityUserLimit, communityUserLimit, false, true, true)
	requireLimit(t, features["project_limit"], communityProjectLimit, communityProjectLimit, false, true, true)
}

func requireLimit(t *testing.T, raw json.RawMessage, expectedLimit, expectedCurrent int64, expectedAllowed, expectedAvailable, expectedReached bool) {
	t.Helper()

	var limit map[string]any
	require.NoError(t, json.Unmarshal(raw, &limit))

	require.Equal(t, float64(expectedLimit), limit["limit"])
	require.Equal(t, float64(expectedCurrent), limit["current"])
	require.Equal(t, expectedAllowed, limit["allowed"])
	require.Equal(t, expectedAvailable, limit["available"])
	require.Equal(t, expectedReached, limit["limit_reached"])
}

type communityOrgRepo struct {
	count int64
}

func (r communityOrgRepo) CreateOrganisation(context.Context, *datastore.Organisation) error {
	return nil
}
func (r communityOrgRepo) UpdateOrganisation(context.Context, *datastore.Organisation) error {
	return nil
}
func (r communityOrgRepo) UpdateOrganisationLicenseData(context.Context, string, string) error {
	return nil
}
func (r communityOrgRepo) DeleteOrganisation(context.Context, string) error { return nil }
func (r communityOrgRepo) FetchOrganisationByID(context.Context, string) (*datastore.Organisation, error) {
	return nil, nil
}
func (r communityOrgRepo) FetchOrganisationByCustomDomain(context.Context, string) (*datastore.Organisation, error) {
	return nil, nil
}
func (r communityOrgRepo) FetchOrganisationByAssignedDomain(context.Context, string) (*datastore.Organisation, error) {
	return nil, nil
}
func (r communityOrgRepo) LoadOrganisationsPaged(context.Context, datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}
func (r communityOrgRepo) LoadOrganisationsPagedWithSearch(context.Context, datastore.Pageable, string) ([]datastore.Organisation, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}
func (r communityOrgRepo) CountOrganisations(context.Context) (int64, error) { return r.count, nil }
func (r communityOrgRepo) CalculateUsage(context.Context, string, time.Time, time.Time) (*datastore.OrganisationUsage, error) {
	return nil, nil
}

type communityUserRepo struct {
	count int64
}

func (r communityUserRepo) CreateUser(context.Context, *datastore.User) error { return nil }
func (r communityUserRepo) UpdateUser(context.Context, *datastore.User) error { return nil }
func (r communityUserRepo) CountUsers(context.Context) (int64, error)         { return r.count, nil }
func (r communityUserRepo) FindUserByEmail(context.Context, string) (*datastore.User, error) {
	return nil, nil
}
func (r communityUserRepo) FindUserByID(context.Context, string) (*datastore.User, error) {
	return nil, nil
}
func (r communityUserRepo) FindUserByToken(context.Context, string) (*datastore.User, error) {
	return nil, nil
}
func (r communityUserRepo) FindUserByEmailVerificationToken(context.Context, string) (*datastore.User, error) {
	return nil, nil
}

type communityProjectRepo struct {
	count int64
}

func (r communityProjectRepo) LoadProjects(context.Context, *datastore.ProjectFilter) ([]*datastore.Project, error) {
	projects := make([]*datastore.Project, r.count)
	for i := range projects {
		projects[i] = &datastore.Project{UID: string(rune('a' + i))}
	}
	return projects, nil
}
func (r communityProjectRepo) CreateProject(context.Context, *datastore.Project) error { return nil }
func (r communityProjectRepo) CountProjects(context.Context) (int64, error)            { return r.count, nil }
func (r communityProjectRepo) UpdateProject(context.Context, *datastore.Project) error { return nil }
func (r communityProjectRepo) DeleteProject(context.Context, string) error             { return nil }
func (r communityProjectRepo) FetchProjectByID(context.Context, string) (*datastore.Project, error) {
	return nil, nil
}
func (r communityProjectRepo) GetProjectsWithEventsInTheInterval(context.Context, int) ([]datastore.ProjectEvents, error) {
	return nil, nil
}
func (r communityProjectRepo) FillProjectsStatistics(context.Context, *datastore.Project) error {
	return nil
}

// mutableProjectRepo lets a test change the set of projects (and force a DB
// error) after the licenser has been built, simulating projects created in
// another process and transient database failures.
type mutableProjectRepo struct {
	communityProjectRepo
	mu     sync.Mutex
	uids   []string
	err    error
	onLoad func()
}

func (r *mutableProjectRepo) set(uids []string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.uids = uids
	r.err = err
}

func (r *mutableProjectRepo) LoadProjects(context.Context, *datastore.ProjectFilter) ([]*datastore.Project, error) {
	r.mu.Lock()
	onLoad := r.onLoad
	err := r.err
	uids := append([]string(nil), r.uids...)
	r.mu.Unlock()

	// Simulate a concurrent project mutation that commits while this read is in
	// flight. Run outside the repo lock so it can take the licenser lock.
	if onLoad != nil {
		onLoad()
	}

	if err != nil {
		return nil, err
	}
	projects := make([]*datastore.Project, 0, len(uids))
	for _, id := range uids {
		projects = append(projects, &datastore.Project{UID: id})
	}
	return projects, nil
}

func (r *mutableProjectRepo) CountProjects(context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return 0, r.err
	}
	return int64(len(r.uids)), nil
}

func newCommunityLicenserForTest(t *testing.T, repo datastore.ProjectRepository) *Licenser {
	t.Helper()
	l, err := NewLicenser(LicenserConfig{
		OrgRepo:     communityOrgRepo{},
		UserRepo:    communityUserRepo{},
		ProjectRepo: repo,
	})
	require.NoError(t, err)
	require.True(t, l.isCommunity.Load())
	return l
}

func (l *Licenser) expireProjectCacheForTest() {
	l.mu.Lock()
	l.projectsFetchedAt = time.Now().Add(-2 * communityProjectCacheTTL)
	l.mu.Unlock()
}

// TestCommunityProjectEnabledRefreshesFromDB proves the worker picks up a
// project created after it started. Without the refresh, the per-process
// snapshot would gate that project's event deliveries forever.
func TestCommunityProjectEnabledRefreshesFromDB(t *testing.T) {
	repo := &mutableProjectRepo{uids: []string{"a"}}
	l := newCommunityLicenserForTest(t, repo)

	require.True(t, l.ProjectEnabled("a"))
	require.False(t, l.ProjectEnabled("b"))

	// A project is created in another process after this licenser started.
	repo.set([]string{"a", "b"}, nil)

	// Within the miss window the cached set is reused (no DB refresh yet).
	require.False(t, l.ProjectEnabled("b"))

	// Once the cache is stale, the next lookup reconciles with the DB.
	l.expireProjectCacheForTest()
	require.True(t, l.ProjectEnabled("b"))
	require.True(t, l.ProjectEnabled("a"))
}

// TestCommunityProjectEnabledEnforcesLimit proves the downgrade case still
// holds: with more projects than the community limit, only the allowed subset
// stays enabled.
func TestCommunityProjectEnabledEnforcesLimit(t *testing.T) {
	repo := &mutableProjectRepo{uids: []string{"a", "b", "c"}}
	l := newCommunityLicenserForTest(t, repo)

	// enforceProjectLimit keeps the last communityProjectLimit projects.
	require.False(t, l.ProjectEnabled("a"))
	require.True(t, l.ProjectEnabled("b"))
	require.True(t, l.ProjectEnabled("c"))
}

// TestCommunityProjectEnabledKeepsCacheOnRefreshError proves the failure policy:
// a transient DB error keeps the last-known set and does not enable an unknown
// project (fail closed), rather than bypassing the project limit.
func TestCommunityProjectEnabledKeepsCacheOnRefreshError(t *testing.T) {
	repo := &mutableProjectRepo{uids: []string{"a"}}
	l := newCommunityLicenserForTest(t, repo)
	require.True(t, l.ProjectEnabled("a"))

	repo.set([]string{"a"}, errors.New("db down"))
	l.expireProjectCacheForTest()

	require.True(t, l.ProjectEnabled("a"))
	require.False(t, l.ProjectEnabled("b"))
}

// activeCircuitBreakingResp is a validation response for an active license that
// grants the circuit_breaking boolean entitlement.
const activeCircuitBreakingResp = `{"status":true,"data":{"valid":true,"status":"active","entitlements":[{"key":"circuit_breaking","value":true}]}}`

// licenseResponder is a mutable HTTP response the fake billing service returns,
// letting a test flip a license from active to suspended/revoked (or a transport
// error) after the licenser has been built.
type licenseResponder struct {
	mu   sync.Mutex
	code int
	body string
}

func (r *licenseResponder) set(code int, body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.code = code
	r.body = body
}

func (r *licenseResponder) get() (int, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.code, r.body
}

func newLicenseTestServer(t *testing.T, resp *licenseResponder) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		code, body := resp.get()
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newLicensedLicenserForTest builds a licensed licenser wired to a fake billing
// server and starts it in the active state. The client uses retryCount 0 so the
// transport-error test does not incur the retry backoff. It bypasses the
// background ticker (validateAndCache is driven directly) so the fail-closed /
// fail-open assertions are deterministic.
func newLicensedLicenserForTest(t *testing.T) (*Licenser, *licenseResponder) {
	t.Helper()

	resp := &licenseResponder{code: http.StatusOK, body: activeCircuitBreakingResp}
	srv := newLicenseTestServer(t, resp)

	client := &Client{
		host:         srv.URL,
		validatePath: "/validate",
		timeout:      2 * time.Second,
		retryCount:   0,
		httpClient:   srv.Client(),
	}

	l := &Licenser{
		client:       client,
		licenseKey:   "test-key",
		cacheTTL:     defaultCacheTTL,
		entitlements: make(map[string]EntitlementValue),
	}

	require.NoError(t, l.validateAndCache(context.Background()))
	require.True(t, l.CircuitBreaking())
	return l, resp
}

// TestLicenserSuspendedFailsClosed proves the fail-closed path: a suspended
// validation clears the cached entitlements, records the status, and the feature
// gate stops serving the premium feature on a live licenser.
func TestLicenserSuspendedFailsClosed(t *testing.T) {
	l, resp := newLicensedLicenserForTest(t)

	resp.set(http.StatusOK, `{"status":true,"data":{"valid":true,"status":"suspended","entitlements":[{"key":"circuit_breaking","value":true}]}}`)

	require.ErrorIs(t, l.validateAndCache(context.Background()), ErrLicenseSuspended)
	require.False(t, l.CircuitBreaking())

	l.entitlementsMu.RLock()
	require.Empty(t, l.entitlements)
	require.Equal(t, licenseStatusSuspended, l.status)
	l.entitlementsMu.RUnlock()
}

// TestLicenserRevokedFailsClosed is the revoked counterpart of the suspended
// fail-closed case.
func TestLicenserRevokedFailsClosed(t *testing.T) {
	l, resp := newLicensedLicenserForTest(t)

	resp.set(http.StatusOK, `{"status":true,"data":{"valid":true,"status":"revoked","entitlements":[{"key":"circuit_breaking","value":true}]}}`)

	require.ErrorIs(t, l.validateAndCache(context.Background()), ErrLicenseRevoked)
	require.False(t, l.CircuitBreaking())

	l.entitlementsMu.RLock()
	require.Empty(t, l.entitlements)
	require.Equal(t, licenseStatusRevoked, l.status)
	l.entitlementsMu.RUnlock()
}

// TestLicenserTransientErrorKeepsLastGood proves the fail-open path: a transport
// error (non-200) keeps the last-good entitlements and does not flip the status,
// so a network blip cannot revoke a valid customer's access.
func TestLicenserTransientErrorKeepsLastGood(t *testing.T) {
	l, resp := newLicensedLicenserForTest(t)

	resp.set(http.StatusInternalServerError, `{}`)

	err := l.validateAndCache(context.Background())
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrLicenseSuspended))
	require.False(t, errors.Is(err, ErrLicenseRevoked))

	require.True(t, l.CircuitBreaking())

	l.entitlementsMu.RLock()
	require.Equal(t, "active", l.status)
	l.entitlementsMu.RUnlock()
}

// TestLicenserActiveRefreshRestoresEntitlements proves a reinstated license
// restores entitlements: after a suspension clears them, a subsequent active
// validation re-grants the feature.
func TestLicenserActiveRefreshRestoresEntitlements(t *testing.T) {
	l, resp := newLicensedLicenserForTest(t)

	resp.set(http.StatusOK, `{"status":true,"data":{"valid":true,"status":"suspended","entitlements":[]}}`)
	require.ErrorIs(t, l.validateAndCache(context.Background()), ErrLicenseSuspended)
	require.False(t, l.CircuitBreaking())

	resp.set(http.StatusOK, activeCircuitBreakingResp)
	require.NoError(t, l.validateAndCache(context.Background()))
	require.True(t, l.CircuitBreaking())

	l.entitlementsMu.RLock()
	require.Equal(t, "active", l.status)
	l.entitlementsMu.RUnlock()
}

// activeTrialResp is a validation response for an ACTIVE self-hosted trial: full
// Premium features with no trial-specific numeric caps.
const activeTrialResp = `{"status":true,"data":{"valid":true,"status":"active","trial":true,"entitlements":[{"key":"circuit_breaking","value":true},{"key":"project_limit","value":-1},{"key":"org_limit","value":-1}]}}`

// trialExpiredResp mirrors what the billing service returns for a lapsed trial: HTTP 400
// with the distinct message the client maps to ErrLicenseTrialExpired.
const trialExpiredResp = `{"status":false,"message":"Trial has expired"}`

// TestParseErrorMapsTrialExpired proves the client wiring for the trial-expired
// signal: parseError maps the distinct message to ErrLicenseTrialExpired while a
// paid "License has expired" still maps to ErrLicenseExpired, and isTrialExpiredBody
// only fires for the trial message on a non-200 body (so paid signalling on that
// branch is untouched).
func TestParseErrorMapsTrialExpired(t *testing.T) {
	c := &Client{}

	require.ErrorIs(t, c.parseError("Trial has expired"), ErrLicenseTrialExpired)
	require.ErrorIs(t, c.parseError("License has expired"), ErrLicenseExpired)
	require.ErrorIs(t, c.parseError("License is suspended"), ErrLicenseSuspended)

	require.True(t, c.isTrialExpiredBody([]byte(trialExpiredResp)))
	require.False(t, c.isTrialExpiredBody([]byte(`{"status":false,"message":"License has expired"}`)))
	require.False(t, c.isTrialExpiredBody([]byte(`{"status":false,"message":"License is suspended"}`)))
	require.False(t, c.isTrialExpiredBody([]byte(activeTrialResp)))
}

// newTrialLicenserForTest builds a licensed licenser wired to a fake billing
// server serving an active trial, with community repos so the post-degradation
// gates have counts to check. The background ticker is bypassed (validateAndCache
// is driven directly) for deterministic assertions.
func newTrialLicenserForTest(t *testing.T) (*Licenser, *licenseResponder) {
	t.Helper()

	resp := &licenseResponder{code: http.StatusOK, body: activeTrialResp}
	srv := newLicenseTestServer(t, resp)

	client := &Client{
		host:         srv.URL,
		validatePath: "/validate",
		timeout:      2 * time.Second,
		retryCount:   0,
		httpClient:   srv.Client(),
	}

	l := &Licenser{
		client:       client,
		licenseKey:   "trial-key",
		cacheTTL:     defaultCacheTTL,
		entitlements: make(map[string]EntitlementValue),
		// org at the community cap (1), project below it (1 < 2), so the degraded
		// gates show the OSS floor: org creation denied, project creation allowed.
		orgRepo:     communityOrgRepo{count: communityOrgLimit},
		userRepo:    communityUserRepo{count: 0},
		projectRepo: communityProjectRepo{count: 1},
	}

	require.NoError(t, l.validateAndCache(context.Background()))
	return l, resp
}

// TestLicenserTrialExpiredDegradesToCommunity proves the runtime half of the
// trial-expiry policy: a live licensed licenser whose trial lapses degrades to the
// community/OSS floor rather than failing closed. Premium features go off, but
// limit gates return community headroom (not the all-deny an empty entitlement map
// would produce with isCommunity == false).
func TestLicenserTrialExpiredDegradesToCommunity(t *testing.T) {
	ctx := context.Background()
	l, resp := newTrialLicenserForTest(t)

	// While the trial is active: full premium feature and unlimited limits.
	require.True(t, l.CircuitBreaking())
	allowed, err := l.CheckProjectLimit(ctx)
	require.NoError(t, err)
	require.True(t, allowed)

	// Trial lapses: billing service returns the distinct expired-trial signal.
	resp.set(http.StatusBadRequest, trialExpiredResp)
	require.ErrorIs(t, l.validateAndCache(ctx), ErrLicenseTrialExpired)
	require.Equal(t, licenseStatusTrialExpired, l.status)

	// Now in community mode, not the fail-closed deny path.
	require.True(t, l.isCommunity.Load())
	require.False(t, l.CircuitBreaking())

	// Community floor: project creation still allowed (1 < 2 headroom), proving
	// it did NOT drop below OSS to all-deny.
	projectAllowed, err := l.CheckProjectLimit(ctx)
	require.NoError(t, err)
	require.True(t, projectAllowed)

	// Community org cap of 1 is enforced (org count already at 1).
	orgAllowed, err := l.CheckOrgLimit(ctx)
	require.NoError(t, err)
	require.False(t, orgAllowed)
}

// TestNewLicenserBootTrialExpiredFallsToCommunity proves the boot half: an
// instance that starts with an already-expired trial key does NOT fail startup;
// NewLicenser returns a community licenser (no error, no premium, community floor).
func TestNewLicenserBootTrialExpiredFallsToCommunity(t *testing.T) {
	ctx := context.Background()
	resp := &licenseResponder{code: http.StatusBadRequest, body: trialExpiredResp}
	srv := newLicenseTestServer(t, resp)

	client := &Client{
		host:         srv.URL,
		validatePath: "/validate",
		timeout:      2 * time.Second,
		retryCount:   0,
		httpClient:   srv.Client(),
	}

	l, err := NewLicenser(LicenserConfig{
		LicenseKey:  "expired-trial-key",
		Client:      client,
		OrgRepo:     communityOrgRepo{count: 0},
		UserRepo:    communityUserRepo{count: 0},
		ProjectRepo: communityProjectRepo{count: 1},
	})
	require.NoError(t, err)
	t.Cleanup(l.Close)

	require.True(t, l.isCommunity.Load())
	require.False(t, l.CircuitBreaking())

	projectAllowed, err := l.CheckProjectLimit(ctx)
	require.NoError(t, err)
	require.True(t, projectAllowed)
}

// TestNewLicenserBootPaidExpiredStillErrors is the regression guard for the boot
// path: a paid definitive-negative (suspended here) must still fail startup, never
// silently degrade to community like an expired trial.
func TestNewLicenserBootPaidSuspendedStillErrors(t *testing.T) {
	resp := &licenseResponder{code: http.StatusOK, body: `{"status":true,"data":{"valid":true,"status":"suspended","entitlements":[]}}`}
	srv := newLicenseTestServer(t, resp)

	client := &Client{
		host:         srv.URL,
		validatePath: "/validate",
		timeout:      2 * time.Second,
		retryCount:   0,
		httpClient:   srv.Client(),
	}

	_, err := NewLicenser(LicenserConfig{
		LicenseKey:  "suspended-key",
		Client:      client,
		OrgRepo:     communityOrgRepo{},
		UserRepo:    communityUserRepo{},
		ProjectRepo: communityProjectRepo{},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrLicenseSuspended)
}

// TestLicenserBackgroundRefreshReflectsSuspension proves the end-to-end fix: a
// running licenser reflects a suspension via the background ticker, without a
// process restart.
func TestLicenserBackgroundRefreshReflectsSuspension(t *testing.T) {
	resp := &licenseResponder{code: http.StatusOK, body: activeCircuitBreakingResp}
	srv := newLicenseTestServer(t, resp)

	client := &Client{
		host:         srv.URL,
		validatePath: "/validate",
		timeout:      2 * time.Second,
		retryCount:   0,
		httpClient:   srv.Client(),
	}

	l, err := NewLicenser(LicenserConfig{
		LicenseKey: "test-key",
		Client:     client,
		CacheTTL:   20 * time.Millisecond,
	})
	require.NoError(t, err)
	t.Cleanup(l.Close)

	require.True(t, l.CircuitBreaking())

	resp.set(http.StatusOK, `{"status":true,"data":{"valid":true,"status":"suspended","entitlements":[]}}`)

	require.Eventually(t, func() bool {
		return !l.CircuitBreaking()
	}, 2*time.Second, 10*time.Millisecond)
}

// TestCommunityProjectEnabledRefreshDoesNotClobberConcurrentMutation proves a
// refresh that read the DB before a concurrent AddEnabledProject committed does
// not overwrite that optimistic mutation. The repo's read returns the stale set
// (without "b"), but "b" is added during the read, so the refresh is discarded.
func TestCommunityProjectEnabledRefreshDoesNotClobberConcurrentMutation(t *testing.T) {
	repo := &mutableProjectRepo{uids: []string{"a"}}
	l := newCommunityLicenserForTest(t, repo)

	repo.mu.Lock()
	repo.onLoad = func() { l.AddEnabledProject("b") }
	repo.mu.Unlock()

	l.expireProjectCacheForTest()

	// The lookup triggers a refresh; the DB read does not include "b", but "b"
	// is added mid-read. The refresh must not drop it.
	require.True(t, l.ProjectEnabled("b"))
	require.True(t, l.ProjectEnabled("a"))
}
