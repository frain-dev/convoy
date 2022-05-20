//go:build integration
// +build integration

package badger

import (
	"context"
	"fmt"
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

func Test_FetchGroupByIDs(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)

	groups := make([]string, 3)
	for i := 0; i < len(groups); i++ {
		group := &datastore.Group{
			Name: uuid.NewString(),
			UID:  uuid.NewString(),
		}
		require.NoError(t, groupRepo.CreateGroup(context.Background(), group))
		groups[i] = group.UID
	}

	grps, err := groupRepo.FetchGroupsByIDs(context.Background(), groups[1:])
	require.NoError(t, err)

	require.Equal(t, 2, len(grps))
}

func Test_CreateGroup(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)

	newOrg := &datastore.Group{
		Name: "Next group",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	// Fetch org again
	org, err := groupRepo.FetchGroupByID(context.Background(), newOrg.UID)
	require.NoError(t, err)

	require.Equal(t, org.UID, newOrg.UID)
}

func Test_LoadGroups(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewGroupRepo(db)

	orgs, err := orgRepo.LoadGroups(context.Background(), &datastore.GroupFilter{})
	require.NoError(t, err)

	require.Equal(t, len(orgs), 0)

	for i := 0; i < 10; i++ {
		g := &datastore.Group{
			Name: "Next group",
			UID:  uuid.NewString(),
		}
		require.NoError(t, orgRepo.CreateGroup(context.Background(), g))
	}

	orgs2, err := orgRepo.LoadGroups(context.Background(), &datastore.GroupFilter{})
	require.NoError(t, err)

	require.Equal(t, len(orgs2), 10)
}

func Test_DeleteGroup(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewGroupRepo(db)

	for i := 0; i < 2; i++ {
		g := &datastore.Group{
			Name: "Next group",
			UID:  fmt.Sprintf("uid-%v", i),
		}
		require.NoError(t, orgRepo.CreateGroup(context.Background(), g))
	}

	require.NoError(t, orgRepo.DeleteGroup(context.Background(), "uid-1"))

	orgs, err := orgRepo.LoadGroups(context.Background(), &datastore.GroupFilter{})
	require.NoError(t, err)

	require.Equal(t, len(orgs), 1)
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
		GroupID: group1.UID,
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

	err = NewEventRepo(db).CreateEvent(context.Background(), event)
	require.NoError(t, err)

	groups := []*datastore.Group{group1, group2}
	err = groupRepo.FillGroupsStatistics(context.Background(), groups)
	require.NoError(t, err)

	require.Equal(t, group1.Statistics, datastore.GroupStatistics{
		GroupID:      "",
		MessagesSent: 1,
		TotalApps:    1,
	})

	require.Equal(t, group1.Statistics, datastore.GroupStatistics{
		GroupID:      "",
		MessagesSent: 0,
		TotalApps:    1,
	})
}
