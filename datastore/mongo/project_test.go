//go:build integration
// +build integration

package mongo

import (
	"context"
	"testing"

	"gopkg.in/guregu/null.v4"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func Test_FetchProjectByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	projectRepo := NewProjectRepo(store)

	newOrg := &datastore.Project{
		Name: "Yet another project",
		UID:  ulid.Make().String(),
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), newOrg))

	// Fetch org again
	org, err := projectRepo.FetchProjectByID(context.Background(), newOrg.UID)
	require.NoError(t, err)

	require.Equal(t, org.UID, newOrg.UID)
}

func Test_CreateProject(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	tt := []struct {
		name        string
		projects    []datastore.Project
		isDuplicate bool
	}{
		{
			name: "create project",
			projects: []datastore.Project{
				{
					Name: "project 1",
					UID:  ulid.Make().String(),
				},
			},
		},

		{
			name: "cannot create project with existing name",
			projects: []datastore.Project{
				{
					Name:           "project 2",
					OrganisationID: "123abc",
					UID:            ulid.Make().String(),
				},

				{
					Name:           "project 2",
					OrganisationID: "123abc",
					UID:            ulid.Make().String(),
				},
			},
			isDuplicate: true,
		},

		{
			name: "can create project with existing name that has been deleted",
			projects: []datastore.Project{
				{
					Name:           "project 3",
					OrganisationID: "abc",
					UID:            ulid.Make().String(),
					DeletedAt:      null.Time{},
				},

				{
					Name:           "project 3",
					OrganisationID: "abc",
					DeletedAt:      null.Time{},
					UID:            ulid.Make().String(),
				},
			},
		},
		{
			name: "can create project with existing name in a different organisation",
			projects: []datastore.Project{
				{
					Name:           "project 4",
					OrganisationID: ulid.Make().String(),
					UID:            ulid.Make().String(),
				},

				{
					Name:           "project 4",
					OrganisationID: ulid.Make().String(),
					UID:            ulid.Make().String(),
				},
			},
			isDuplicate: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			projectRepo := NewProjectRepo(store)

			for i, project := range tc.projects {
				newProject := &datastore.Project{
					Name:      project.Name,
					UID:       project.UID,
					DeletedAt: project.DeletedAt,
				}

				if i == 0 {
					require.NoError(t, projectRepo.CreateProject(context.Background(), newProject))
				}

				if i > 0 && tc.isDuplicate {
					err := projectRepo.CreateProject(context.Background(), newProject)
					require.Error(t, err)
					require.ErrorIs(t, err, datastore.ErrDuplicateProjectName)
				}

				if i > 0 && !tc.isDuplicate {
					require.NoError(t, projectRepo.CreateProject(context.Background(), newProject))
				}
			}
		})
	}
}

func Test_LoadProjects(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	store := getStore(db)

	orgRepo := NewProjectRepo(store)

	orgs, err := orgRepo.LoadProjects(context.Background(), &datastore.ProjectFilter{})
	require.NoError(t, err)

	require.True(t, len(orgs) == 0)
}

func Test_FillProjectStatistics(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	projectRepo := NewProjectRepo(store)

	project1 := &datastore.Project{
		Name: "project1",
		UID:  ulid.Make().String(),
	}

	project2 := &datastore.Project{
		Name: "project2",
		UID:  ulid.Make().String(),
	}

	err := projectRepo.CreateProject(context.Background(), project1)
	require.NoError(t, err)

	err = projectRepo.CreateProject(context.Background(), project2)
	require.NoError(t, err)

	endpoint1 := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project1.UID,
	}

	endpoint2 := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project2.UID,
	}

	endpointRepo := NewEndpointRepo(getStore(db))
	endpointCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EndpointCollection)
	err = endpointRepo.CreateEndpoint(endpointCtx, endpoint1, project1.UID)
	require.NoError(t, err)

	err = endpointRepo.CreateEndpoint(endpointCtx, endpoint2, project2.UID)
	require.NoError(t, err)

	event := &datastore.Event{
		UID:       ulid.Make().String(),
		ProjectID: endpoint1.ProjectID,
		Endpoints: []string{endpoint1.UID},
	}

	eventCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EventCollection)
	err = NewEventRepository(store).CreateEvent(eventCtx, event)
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
