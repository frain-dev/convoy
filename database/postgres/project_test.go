//go:build integration
// +build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dchest/uniuri"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"
)

func Test_FetchProjectByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	projectRepo := NewProjectRepo(db, nil)

	newProject := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Yet another project",
		LogoURL:        "s3.com/dsiuirueiy",
		OrganisationID: org.UID,
		Type:           datastore.IncomingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), newProject))

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

	projectRepo := NewProjectRepo(db, nil)

	org := seedOrg(t, db)

	const name = "test_project"

	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           name,
		OrganisationID: org.UID,
		Type:           datastore.IncomingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	err := projectRepo.CreateProject(context.Background(), project)
	require.NoError(t, err)
	require.NotEmpty(t, project.ProjectConfigID)

	projectWithExistingName := &datastore.Project{
		UID:            ulid.Make().String(),
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
		UID:            ulid.Make().String(),
		Name:           name,
		OrganisationID: seedOrg(t, db).UID,
		Config:         &datastore.DefaultProjectConfig,
	}

	// should create project with same name in diff org
	err = projectRepo.CreateProject(context.Background(), projectInDiffOrg)
	require.NoError(t, err)
}

func Test_UpdateProject(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	projectRepo := NewProjectRepo(db, nil)

	org := seedOrg(t, db)

	const name = "test_project"

	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           name,
		OrganisationID: org.UID,
		Config:         &datastore.DefaultProjectConfig,
	}

	err := projectRepo.CreateProject(context.Background(), project)
	require.NoError(t, err)

	updatedProject := &datastore.Project{
		UID:             project.UID,
		Name:            "convoy",
		LogoURL:         "https:/oilvmm.com",
		OrganisationID:  project.OrganisationID,
		ProjectConfigID: project.ProjectConfigID, // TODO(all): if i comment this line this test never exits, weird problem
		Config: &datastore.ProjectConfig{
			MaxIngestSize:            8483,
			ReplayAttacks:            true,
			IsRetentionPolicyEnabled: true,
			RetentionPolicy:          &datastore.RetentionPolicyConfiguration{Policy: "99d"},
			RateLimit: &datastore.RateLimitConfiguration{
				Count:    8773,
				Duration: 7766,
			},
			Strategy: &datastore.StrategyConfiguration{
				Type:       datastore.ExponentialStrategyProvider,
				Duration:   2434,
				RetryCount: 5737,
			},
			Signature: &datastore.SignatureConfiguration{
				Header: "f888fbfb",
				Versions: []datastore.SignatureVersion{
					{
						UID:       ulid.Make().String(),
						Hash:      "SHA512",
						Encoding:  datastore.HexEncoding,
						CreatedAt: time.Now(),
					},
				},
			},
			MetaEvent: &datastore.MetaEventConfiguration{
				IsEnabled: false,
			},
		},
		RetainedEvents: 300,
	}

	err = projectRepo.UpdateProject(context.Background(), updatedProject)
	require.NoError(t, err)

	dbProject, err := projectRepo.FetchProjectByID(context.Background(), project.UID)
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

	for i := range updatedProject.Config.Signature.Versions {
		version := &updatedProject.Config.Signature.Versions[i]
		require.NotEmpty(t, version.CreatedAt)
		version.CreatedAt = time.Time{}
	}

	require.Equal(t, updatedProject, dbProject)
}

func Test_LoadProjects(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	projectRepo := NewProjectRepo(db, nil)

	for i := 0; i < 3; i++ {
		project := &datastore.Project{
			UID:            ulid.Make().String(),
			Name:           fmt.Sprintf("%s-project", ulid.Make().String()),
			OrganisationID: org.UID,
			Config:         &datastore.DefaultProjectConfig,
		}

		err := projectRepo.CreateProject(context.Background(), project)
		require.NoError(t, err)
	}

	for i := 0; i < 4; i++ {
		project := &datastore.Project{
			UID:            ulid.Make().String(),
			Name:           fmt.Sprintf("%s-project", ulid.Make().String()),
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
	projectRepo := NewProjectRepo(db, nil)

	project1 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "project1",
		Config:         &datastore.DefaultProjectConfig,
		OrganisationID: org.UID,
	}

	project2 := &datastore.Project{
		UID:            ulid.Make().String(),
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
		UID:       ulid.Make().String(),
		ProjectID: project2.UID,
		Secrets: datastore.Secrets{
			{UID: ulid.Make().String()},
		},
	}

	endpointRepo := NewEndpointRepo(db, nil)
	err = endpointRepo.CreateEndpoint(context.Background(), endpoint1, project1.UID)
	require.NoError(t, err)

	err = endpointRepo.CreateEndpoint(context.Background(), endpoint2, project2.UID)
	require.NoError(t, err)

	source1 := &datastore.Source{
		UID:       ulid.Make().String(),
		ProjectID: project1.UID,
		Name:      "Convoy-Prod",
		MaskID:    uniuri.NewLen(16),
		Type:      datastore.HTTPSource,
		Verifier:  &datastore.VerifierConfig{},
	}

	err = NewSourceRepo(db, nil).CreateSource(context.Background(), source1)
	require.NoError(t, err)

	subscription := &datastore.Subscription{
		UID:         ulid.Make().String(),
		Name:        "Subscription",
		Type:        datastore.SubscriptionTypeAPI,
		ProjectID:   project2.UID,
		EndpointID:  endpoint1.UID,
		AlertConfig: &datastore.DefaultAlertConfig,
		RetryConfig: &datastore.DefaultRetryConfig,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"some.event"},
			Filter: datastore.FilterSchema{
				Headers: datastore.M{},
				Body:    datastore.M{},
			},
		},
	}

	err = NewSubscriptionRepo(db, nil).CreateSubscription(context.Background(), project2.UID, subscription)
	require.NoError(t, err)

	err = projectRepo.FillProjectsStatistics(context.Background(), project1)
	require.NoError(t, err)

	require.Equal(t, datastore.ProjectStatistics{
		MessagesSent:       0,
		TotalEndpoints:     1,
		TotalSources:       1,
		TotalSubscriptions: 0,
	}, *project1.Statistics)

	err = projectRepo.FillProjectsStatistics(context.Background(), project2)
	require.NoError(t, err)

	require.Equal(t, datastore.ProjectStatistics{
		MessagesSent:       0,
		TotalEndpoints:     1,
		TotalSources:       0,
		TotalSubscriptions: 1,
	}, *project2.Statistics)
}

func Test_DeleteProject(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	projectRepo := NewProjectRepo(db, nil)

	project := &datastore.Project{
		UID:            ulid.Make().String(),
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

	endpointRepo := NewEndpointRepo(db, nil)
	err = endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	event := &datastore.Event{
		ProjectID: endpoint.ProjectID,
		Endpoints: []string{endpoint.UID},
		Headers:   httpheader.HTTPHeader{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = NewEventRepo(db, nil).CreateEvent(context.Background(), event)
	require.NoError(t, err)

	sub := &datastore.Subscription{
		Name:        "test_sub",
		Type:        datastore.SubscriptionTypeAPI,
		ProjectID:   project.UID,
		AlertConfig: &datastore.DefaultAlertConfig,
		RetryConfig: &datastore.DefaultRetryConfig,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
			Filter: datastore.FilterSchema{
				Headers: datastore.M{},
				Body:    datastore.M{},
			},
		},
		RateLimitConfig: &datastore.DefaultRateLimitConfig,
	}

	err = NewSubscriptionRepo(db, nil).CreateSubscription(context.Background(), project.UID, sub)
	require.NoError(t, err)

	err = projectRepo.DeleteProject(context.Background(), project.UID)
	require.NoError(t, err)

	_, err = projectRepo.FetchProjectByID(context.Background(), project.UID)
	require.Equal(t, datastore.ErrProjectNotFound, err)

	_, err = NewEventRepo(db, nil).FindEventByID(context.Background(), event.ProjectID, event.UID)
	require.Equal(t, datastore.ErrEventNotFound, err)

	_, err = NewEndpointRepo(db, nil).FindEndpointByID(context.Background(), project.UID, endpoint.UID)
	require.Equal(t, datastore.ErrEndpointNotFound, err)

	_, err = NewSubscriptionRepo(db, nil).FindSubscriptionByID(context.Background(), project.UID, sub.UID)
	require.Equal(t, datastore.ErrSubscriptionNotFound, err)
}

func seedOrg(t *testing.T, db database.Database) *datastore.Organisation {
	user := seedUser(t, db)

	org := &datastore.Organisation{
		UID:            ulid.Make().String(),
		Name:           ulid.Make().String() + "-new_org",
		OwnerID:        user.UID,
		CustomDomain:   null.NewString(ulid.Make().String(), true),
		AssignedDomain: null.NewString(ulid.Make().String(), true),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := NewOrgRepo(db, nil).CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	return org
}

func seedProject(t *testing.T, db database.Database) *datastore.Project {
	p := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Yet another project",
		LogoURL:        "s3.com/dsiuirueiy",
		OrganisationID: seedOrg(t, db).UID,
		Type:           datastore.IncomingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	err := NewProjectRepo(db, nil).CreateProject(context.Background(), p)
	require.NoError(t, err)

	return p
}
