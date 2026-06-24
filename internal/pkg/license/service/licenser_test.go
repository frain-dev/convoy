package service

import (
	"context"
	"encoding/json"
	"errors"
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
	require.True(t, l.isCommunity)
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
