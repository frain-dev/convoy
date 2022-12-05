//go:build integration
// +build integration

package mongo

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_FetchGroupByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	projectRepo := NewProjectRepo(store)

	newOrg := &datastore.Project{
		Name: "Yet another group",
		UID:  uuid.NewString(),
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), newOrg))

	// Fetch org again
	org, err := projectRepo.FetchProjectByID(context.Background(), newOrg.UID)
	require.NoError(t, err)

	require.Equal(t, org.UID, newOrg.UID)
}

func Test_CreateGroup(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	d := primitive.NewDateTimeFromTime(time.Now())

	tt := []struct {
		name        string
		groups      []datastore.Project
		isDuplicate bool
	}{
		{
			name: "create group",
			groups: []datastore.Project{
				{
					Name: "group 1",
					UID:  uuid.NewString(),
				},
			},
		},

		{
			name: "cannot create group with existing name",
			groups: []datastore.Project{
				{
					Name:           "group 2",
					OrganisationID: "123abc",
					UID:            uuid.NewString(),
				},

				{
					Name:           "group 2",
					OrganisationID: "123abc",
					UID:            uuid.NewString(),
				},
			},
			isDuplicate: true,
		},

		{
			name: "can create group with existing name that has been deleted",
			groups: []datastore.Project{
				{
					Name:           "group 3",
					OrganisationID: "abc",
					UID:            uuid.NewString(),
					DeletedAt:      &d,
				},

				{
					Name:           "group 3",
					OrganisationID: "abc",
					DeletedAt:      nil,
					UID:            uuid.NewString(),
				},
			},
		},
		{
			name: "can create group with existing name in a different organisation",
			groups: []datastore.Project{
				{
					Name:           "group 4",
					OrganisationID: uuid.NewString(),
					UID:            uuid.NewString(),
				},

				{
					Name:           "group 4",
					OrganisationID: uuid.NewString(),
					UID:            uuid.NewString(),
				},
			},
			isDuplicate: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			projectRepo := NewProjectRepo(store)

			for i, group := range tc.groups {
				newGroup := &datastore.Project{
					Name:      group.Name,
					UID:       group.UID,
					DeletedAt: group.DeletedAt,
				}

				if i == 0 {
					require.NoError(t, projectRepo.CreateProject(context.Background(), newGroup))
				}

				if i > 0 && tc.isDuplicate {
					err := projectRepo.CreateProject(context.Background(), newGroup)
					require.Error(t, err)
					require.ErrorIs(t, err, datastore.ErrDuplicateGroupName)
				}

				if i > 0 && !tc.isDuplicate {
					require.NoError(t, projectRepo.CreateProject(context.Background(), newGroup))
				}
			}
		})
	}
}

func Test_LoadGroups(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	store := getStore(db)

	orgRepo := NewProjectRepo(store)

	orgs, err := orgRepo.LoadProjects(context.Background(), &datastore.GroupFilter{})
	require.NoError(t, err)

	require.True(t, len(orgs) == 0)
}

func Test_FillGroupsStatistics(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	projectRepo := NewProjectRepo(store)

	group1 := &datastore.Project{
		Name: "group1",
		UID:  uuid.NewString(),
	}

	group2 := &datastore.Project{
		Name: "group2",
		UID:  uuid.NewString(),
	}

	err := projectRepo.CreateProject(context.Background(), group1)
	require.NoError(t, err)

	err = projectRepo.CreateProject(context.Background(), group2)
	require.NoError(t, err)

	endpoint1 := &datastore.Endpoint{
		UID:     uuid.NewString(),
		GroupID: group1.UID,
	}

	endpoint2 := &datastore.Endpoint{
		UID:     uuid.NewString(),
		GroupID: group2.UID,
	}

	endpointRepo := NewEndpointRepo(getStore(db))
	endpointCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EndpointCollection)
	err = endpointRepo.CreateEndpoint(endpointCtx, endpoint1, group1.UID)
	require.NoError(t, err)

	err = endpointRepo.CreateEndpoint(endpointCtx, endpoint2, group2.UID)
	require.NoError(t, err)

	event := &datastore.Event{
		UID:       uuid.NewString(),
		GroupID:   endpoint1.GroupID,
		Endpoints: []string{endpoint1.UID},
	}

	eventCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EventCollection)
	err = NewEventRepository(store).CreateEvent(eventCtx, event)
	require.NoError(t, err)

	groups := []*datastore.Project{group1, group2}
	err = projectRepo.FillProjectsStatistics(context.Background(), groups)
	require.NoError(t, err)

	require.Equal(t, datastore.GroupStatistics{
		GroupID:      group1.UID,
		MessagesSent: 1,
		TotalApps:    1,
	}, *group1.Statistics)

	require.Equal(t, datastore.GroupStatistics{
		GroupID:      group2.UID,
		MessagesSent: 0,
		TotalApps:    1,
	}, *group2.Statistics)
}
