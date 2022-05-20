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

	groupRepo := NewGroupRepo(db)

	newOrg := &datastore.Group{
		Name: "Yet another group",
		UID:  uuid.NewString(),
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
					UID:            uuid.NewString(),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},

				{
					Name:           "group 2",
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
					UID:            uuid.NewString(),
					DocumentStatus: datastore.DeletedDocumentStatus,
				},

				{
					Name:           "group 3",
					UID:            uuid.NewString(),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			groupRepo := NewGroupRepo(db)

			for i, group := range tc.groups {
				newOrg := &datastore.Group{
					Name:           group.Name,
					UID:            group.UID,
					DocumentStatus: group.DocumentStatus,
				}

				if i == 0 {
					require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

					org, err := groupRepo.FetchGroupByID(context.Background(), newOrg.UID)
					require.NoError(t, err)
					require.Equal(t, org.UID, newOrg.UID)
				}

				if i > 0 && tc.isDuplicate {
					err := groupRepo.CreateGroup(context.Background(), newOrg)
					require.Error(t, err)
				}

				if i > 0 && !tc.isDuplicate {
					require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))
				}
			}

		})
	}
}

func Test_LoadGroups(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewGroupRepo(db)

	orgs, err := orgRepo.LoadGroups(context.Background(), &datastore.GroupFilter{})
	require.NoError(t, err)

	require.True(t, len(orgs) > 0)
}

func Test_FillGroupsStatistics(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)

	group1 := &datastore.Group{
		Name: "group1",
		UID:  uuid.NewString(),
	}

	group2 := &datastore.Group{
		Name: "group2",
		UID:  uuid.NewString(),
	}

	err := groupRepo.CreateGroup(context.Background(), group1)
	require.NoError(t, err)

	err = groupRepo.CreateGroup(context.Background(), group2)
	require.NoError(t, err)

	app1 := &datastore.Application{
		UID:     uuid.NewString(),
		GroupID: group1.UID,
	}

	app2 := &datastore.Application{
		UID:     uuid.NewString(),
		GroupID: group2.UID,
	}

	appRepo := NewApplicationRepo(db)
	err = appRepo.CreateApplication(context.Background(), app1)
	require.NoError(t, err)

	err = appRepo.CreateApplication(context.Background(), app2)
	require.NoError(t, err)

	event := &datastore.Event{
		UID:         uuid.NewString(),
		AppMetadata: &datastore.AppMetadata{UID: app1.UID, GroupID: app1.GroupID},
	}

	err = NewEventRepository(db).CreateEvent(context.Background(), event)
	require.NoError(t, err)

	groups := []*datastore.Group{group1, group2}
	err = groupRepo.FillGroupsStatistics(context.Background(), groups)
	require.NoError(t, err)

	require.Equal(t, *group1.Statistics, datastore.GroupStatistics{
		GroupID:      "",
		MessagesSent: 1,
		TotalApps:    1,
	})

	require.Equal(t, *group2.Statistics, datastore.GroupStatistics{
		GroupID:      "",
		MessagesSent: 0,
		TotalApps:    1,
	})
}
