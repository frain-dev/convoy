//go:build integration
// +build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/frain-dev/convoy/pkg/httpheader"

	"github.com/jmoiron/sqlx"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_FetchProjectByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	projectRepo := NewProjectRepo(db)

	newProject := &datastore.Project{
		Name:           "Yet another project",
		LogoURL:        "s3.com/dsiuirueiy",
		OrganisationID: org.UID,
		Type:           datastore.IncomingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), newProject))
	require.NotEmpty(t, newProject.UID)

	dbProject, err := projectRepo.FetchProjectByID(context.Background(), newProject.UID)
	require.NoError(t, err)

	require.NotEmpty(t, dbProject.CreatedAt)
	require.NotEmpty(t, dbProject.UpdatedAt)

	dbProject.CreatedAt = time.Time{}
	dbProject.UpdatedAt = time.Time{}
	for i := range dbProject.Config.Signature.Versions {
		version := &dbProject.Config.Signature.Versions[i]
		require.NotEmpty(t, version.CreatedAt)
		version.CreatedAt = time.Time{}
	}

	for i := range newProject.Config.Signature.Versions {
		version := &newProject.Config.Signature.Versions[i]
		require.NotEmpty(t, version.CreatedAt)
		version.CreatedAt = time.Time{}
	}

	require.Equal(t, newProject, dbProject)
}

func Test_CreateProject(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	projectRepo := NewProjectRepo(db)

	org := seedOrg(t, db)

	const name = "test_project"

	project := &datastore.Project{
		Name:           name,
		OrganisationID: org.UID,
		Config:         &datastore.DefaultProjectConfig,
	}

	err := projectRepo.CreateProject(context.Background(), project)
	require.NoError(t, err)

	projectWithExistingName := &datastore.Project{
		Name:           name,
		OrganisationID: org.UID,
		Config:         &datastore.DefaultProjectConfig,
	}

	// should not create project with same name
	err = projectRepo.CreateProject(context.Background(), projectWithExistingName)
	require.Equal(t, datastore.ErrDuplicateProjectName, err)

	// delete exisiting project
	err = projectRepo.DeleteProject(context.Background(), project.UID)
	require.NoError(t, err)

	// can now create project with same name
	err = projectRepo.CreateProject(context.Background(), projectWithExistingName)
	require.NoError(t, err)

	projectInDiffOrg := &datastore.Project{
		Name:           name,
		OrganisationID: seedOrg(t, db).UID,
		Config:         &datastore.DefaultProjectConfig,
	}

	// should create project with same name in diff org
	err = projectRepo.CreateProject(context.Background(), projectInDiffOrg)
	require.NoError(t, err)
}

func Test_LoadProjects(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	projectRepo := NewProjectRepo(db)

	for i := 0; i < 3; i++ {
		project := &datastore.Project{
			Name:           fmt.Sprintf("%s-project", uuid.NewString()),
			OrganisationID: org.UID,
			Config:         &datastore.DefaultProjectConfig,
		}

		err := projectRepo.CreateProject(context.Background(), project)
		require.NoError(t, err)
	}

	for i := 0; i < 4; i++ {
		project := &datastore.Project{
			Name:           fmt.Sprintf("%s-project", uuid.NewString()),
			OrganisationID: seedOrg(t, db).UID,
			Config:         &datastore.DefaultProjectConfig,
		}

		err := projectRepo.CreateProject(context.Background(), project)
		require.NoError(t, err)
	}

	projects, err := projectRepo.LoadProjects(context.Background(), &datastore.ProjectFilter{OrgID: org.UID})
	require.NoError(t, err)

	require.True(t, len(projects) == 3)
}

func Test_FillProjectStatistics(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	projectRepo := NewProjectRepo(db)

	project1 := &datastore.Project{
		Name:           "project1",
		Config:         &datastore.DefaultProjectConfig,
		OrganisationID: org.UID,
	}

	project2 := &datastore.Project{
		Name:           "project2",
		Config:         &datastore.DefaultProjectConfig,
		OrganisationID: org.UID,
	}

	err := projectRepo.CreateProject(context.Background(), project1)
	require.NoError(t, err)

	err = projectRepo.CreateProject(context.Background(), project2)
	require.NoError(t, err)

	endpoint1 := &datastore.Endpoint{
		ProjectID: project1.UID,
		TargetURL: "http://google.com",
		Title:     "test_endpoint",
		Secrets: []datastore.Secret{
			{
				Value:     "12345",
				ExpiresAt: null.Time{},
			},
		},
		HttpTimeout:       "10s",
		RateLimit:         3000,
		Events:            0,
		Status:            "",
		RateLimitDuration: "",
		Authentication:    nil,
		CreatedAt:         time.Time{},
		UpdatedAt:         time.Time{},
		DeletedAt:         null.Time{},
	}

	endpoint2 := &datastore.Endpoint{
		ProjectID: project2.UID,
	}

	endpointRepo := NewEndpointRepo(db)
	err = endpointRepo.CreateEndpoint(context.Background(), endpoint1, project1.UID)
	require.NoError(t, err)

	err = endpointRepo.CreateEndpoint(context.Background(), endpoint2, project2.UID)
	require.NoError(t, err)

	event := &datastore.Event{
		ProjectID: endpoint1.ProjectID,
		Endpoints: []string{endpoint1.UID},
		Headers:   httpheader.HTTPHeader{},
	}

	err = NewEventRepo(db).CreateEvent(context.Background(), event)
	require.NoError(t, err)

	err = projectRepo.FillProjectsStatistics(context.Background(), project1)
	require.NoError(t, err)

	require.Equal(t, datastore.ProjectStatistics{
		MessagesSent:   1,
		TotalEndpoints: 1,
	}, *project1.Statistics)

	err = projectRepo.FillProjectsStatistics(context.Background(), project2)
	require.NoError(t, err)

	require.Equal(t, datastore.ProjectStatistics{
		MessagesSent:   0,
		TotalEndpoints: 1,
	}, *project2.Statistics)
}

func Test_DeleteProject(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	projectRepo := NewProjectRepo(db)

	project := &datastore.Project{
		Name:           "project",
		Config:         &datastore.DefaultProjectConfig,
		OrganisationID: org.UID,
	}

	err := projectRepo.CreateProject(context.Background(), project)
	require.NoError(t, err)

	endpoint := &datastore.Endpoint{
		ProjectID: project.UID,
		TargetURL: "http://google.com",
		Title:     "test_endpoint",
		Secrets: []datastore.Secret{
			{
				Value:     "12345",
				ExpiresAt: null.Time{},
			},
		},
		HttpTimeout: "10s",
		RateLimit:   3000,
	}

	endpointRepo := NewEndpointRepo(db)
	err = endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	event := &datastore.Event{
		ProjectID: endpoint.ProjectID,
		Endpoints: []string{endpoint.UID},
		Headers:   httpheader.HTTPHeader{},
	}

	err = NewEventRepo(db).CreateEvent(context.Background(), event)
	require.NoError(t, err)

	sub := &datastore.Subscription{
		Name:        "test_sub",
		Type:        datastore.SubscriptionTypeAPI,
		ProjectID:   project.UID,
		AlertConfig: &datastore.DefaultAlertConfig,
		RetryConfig: &datastore.DefaultRetryConfig,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
			Filter:     datastore.FilterSchema{},
		},
		RateLimitConfig: &datastore.DefaultRateLimitConfig,
	}

	err = NewSubscriptionRepo(db).CreateSubscription(context.Background(), project.UID, sub)
	require.NoError(t, err)

	err = projectRepo.DeleteProject(context.Background(), project.UID)
	require.NoError(t, err)

	_, err = projectRepo.FetchProjectByID(context.Background(), project.UID)
	require.Equal(t, datastore.ErrProjectNotFound, err)

	_, err = NewEventRepo(db).FindEventByID(context.Background(), event.UID)
	require.Equal(t, datastore.ErrEventNotFound, err)

	_, err = NewEndpointRepo(db).FindEndpointByID(context.Background(), project.UID, endpoint.UID)
	require.Equal(t, datastore.ErrEndpointNotFound, err)

	_, err = NewSubscriptionRepo(db).FindSubscriptionByID(context.Background(), project.UID, sub.UID)
	require.Equal(t, datastore.ErrSubscriptionNotFound, err)
}

func seedOrg(t *testing.T, db *sqlx.DB) *datastore.Organisation {
	user := seedUser(t, db)

	org := &datastore.Organisation{
		Name:           uuid.NewString() + "-new_org",
		OwnerID:        user.UID,
		CustomDomain:   null.NewString("https://google.com", true),
		AssignedDomain: null.NewString("https://google.com", true),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := NewOrgRepo(db).CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	return org
}
