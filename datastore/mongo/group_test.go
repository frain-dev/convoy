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

	groupRepo := NewGroupRepo(store)

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

	store := getStore(db)

	d := primitive.NewDateTimeFromTime(time.Now())

	tt := []struct {
		name        string
		groups      []datastore.Group
		isDuplicate bool
	}{
		{
			name: "create group",
			groups: []datastore.Group{
				{
					Name: "group 1",
					UID:  uuid.NewString(),
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
			groups: []datastore.Group{
				{
					Name:           "group 3",
					OrganisationID: "abc",
					UID:            uuid.NewString(),
					DeletedAt:      &d,
				},

				{
					Name:           "group 3",
					OrganisationID: "abc",
					UID:            uuid.NewString(),
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
			groupRepo := NewGroupRepo(store)

			for i, group := range tc.groups {
				newGroup := &datastore.Group{
					Name: group.Name,
					UID:  group.UID,
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

	appRepo := NewApplicationRepo(getStore(db))
	appCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.AppCollection)
	err = appRepo.CreateApplication(appCtx, app1, group1.UID)
	require.NoError(t, err)

	err = appRepo.CreateApplication(appCtx, app2, group2.UID)
	require.NoError(t, err)

	event := &datastore.Event{
		UID:     uuid.NewString(),
		GroupID: app1.GroupID,
		AppID:   app1.UID,
	}

	eventCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EventCollection)
	err = NewEventRepository(store).CreateEvent(eventCtx, event)
	require.NoError(t, err)

	groups := []*datastore.Group{group1, group2}
	err = groupRepo.FillGroupsStatistics(context.Background(), groups)
	require.NoError(t, err)

	require.Equal(t, *group1.Statistics, datastore.GroupStatistics{
		GroupID:      group1.UID,
		MessagesSent: 1,
		TotalApps:    1,
	})

	require.Equal(t, *group2.Statistics, datastore.GroupStatistics{
		GroupID:      group2.UID,
		MessagesSent: 0,
		TotalApps:    1,
	})
}
