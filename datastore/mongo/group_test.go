//go:build integration
// +build integration

package mongo

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_FetchGroupByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	groupRepo := NewGroupRepo(store)

	newOrg := &datastore.Group{
		Name:           "Yet another group",
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	// Fetch org again
	org, err := groupRepo.FetchGroupByID(context.Background(), newOrg.UID)
	require.NoError(t, err)

	require.Equal(t, org.UID, newOrg.UID)
}

func Test_CreateGroup(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	tt := []struct {
		name        string
		groups      []datastore.Group
		isDuplicate bool
	}{
		{
			name: "create group",
			groups: []datastore.Group{
				{
					Name:           "group 1",
					UID:            uuid.NewString(),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
		},

		{
			name: "cannot create group with existing name",
			groups: []datastore.Group{
				{
					Name:           "group 2",
					OrganisationID: "123abc",
					UID:            uuid.NewString(),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},

				{
					Name:           "group 2",
					OrganisationID: "123abc",
					UID:            uuid.NewString(),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			isDuplicate: true,
		},

		{
			name: "can create group with existing name that has been deleted",
			groups: []datastore.Group{
				{
					Name:           "group 3",
					OrganisationID: "abc",
					UID:            uuid.NewString(),
					DocumentStatus: datastore.DeletedDocumentStatus,
				},

				{
					Name:           "group 3",
					OrganisationID: "abc",
					UID:            uuid.NewString(),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
		},
		{
			name: "can create group with existing name in a different organisation",
			groups: []datastore.Group{
				{
					Name:           "group 4",
					OrganisationID: uuid.NewString(),
					UID:            uuid.NewString(),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},

				{
					Name:           "group 4",
					OrganisationID: uuid.NewString(),
					UID:            uuid.NewString(),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			isDuplicate: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			groupRepo := NewGroupRepo(store)

			for i, group := range tc.groups {
				newGroup := &datastore.Group{
					Name:           group.Name,
					UID:            group.UID,
					DocumentStatus: group.DocumentStatus,
				}

				if i == 0 {
					require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))
				}

				if i > 0 && tc.isDuplicate {
					err := groupRepo.CreateGroup(context.Background(), newGroup)
					require.Error(t, err)
					require.ErrorIs(t, err, datastore.ErrDuplicateGroupName)
				}

				if i > 0 && !tc.isDuplicate {
					require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))
				}
			}

		})
	}
}

func Test_LoadGroups(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	store := getStore(db)

	orgRepo := NewGroupRepo(store)

	orgs, err := orgRepo.LoadGroups(context.Background(), &datastore.GroupFilter{})
	require.NoError(t, err)

	require.True(t, len(orgs) == 0)
}

func Test_FillGroupsStatistics(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	groupRepo := NewGroupRepo(store)

	group1 := &datastore.Group{
		Name:           "group1",
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	group2 := &datastore.Group{
		Name:           "group2",
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err := groupRepo.CreateGroup(context.Background(), group1)
	require.NoError(t, err)

	err = groupRepo.CreateGroup(context.Background(), group2)
	require.NoError(t, err)

	endpoint1 := &datastore.Endpoint{
		UID:            uuid.NewString(),
		GroupID:        group1.UID,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	endpoint2 := &datastore.Endpoint{
		UID:            uuid.NewString(),
		GroupID:        group2.UID,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	endpointRepo := NewEndpointRepo(getStore(db))
	endpointCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EndpointCollection)
	err = endpointRepo.CreateEndpoint(endpointCtx, endpoint1, group1.UID)
	require.NoError(t, err)

	err = endpointRepo.CreateEndpoint(endpointCtx, endpoint2, group2.UID)
	require.NoError(t, err)

	event := &datastore.Event{
		UID:            uuid.NewString(),
		GroupID:        endpoint1.GroupID,
		Endpoints:      []string{endpoint1.UID},
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	eventCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EventCollection)
	err = NewEventRepository(store).CreateEvent(eventCtx, event)
	require.NoError(t, err)

	groups := []*datastore.Group{group1, group2}
	err = groupRepo.FillGroupsStatistics(context.Background(), groups)
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
