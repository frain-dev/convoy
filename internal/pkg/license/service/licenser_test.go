package service

import (
	"context"
	"encoding/json"
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
