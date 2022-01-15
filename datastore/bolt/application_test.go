package bolt

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/frain-dev/convoy/datastore"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_LoadApplicationsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	newOrg := &datastore.Group{
		Name: "Group 1",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	for i := 0; i < 10; i++ {
		a := &datastore.Application{
			Title:   fmt.Sprintf("Application %v", i),
			GroupID: newOrg.UID,
			UID:     uuid.NewString(),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	_, pageData, err := appRepo.LoadApplicationsPaged(context.Background(), "", datastore.Pageable{
		Page:    1,
		PerPage: 3,
	})

	require.NoError(t, err)

	require.Equal(t, pageData.TotalPage, int64(4))
}

func Test_CreateApplication(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	newOrg := &datastore.Group{
		Name: "Group 1",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	app := &datastore.Application{
		Title:   "Application 1",
		GroupID: newOrg.UID,
		UID:     uuid.NewString(),
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))
}

func Test_UpdateApplication(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	newGroup := &datastore.Group{
		Name: "Random new group",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))

	app := &datastore.Application{
		UID:     uuid.NewString(),
		Title:   "Next application name",
		GroupID: newGroup.UID,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))

	newTitle := "Newer name"

	app.Title = newTitle

	require.NoError(t, appRepo.UpdateApplication(context.Background(), app))

	newApp, err := appRepo.FindApplicationByID(context.Background(), app.UID)
	require.NoError(t, err)

	require.Equal(t, newTitle, newApp.Title)
}

func Test_FindApplicationByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(db)

	_, err := appRepo.FindApplicationByID(context.Background(), uuid.New().String())
	require.Error(t, err)

	require.True(t, errors.Is(err, datastore.ErrApplicationNotFound))

	groupRepo := NewGroupRepo(db)

	newGroup := &datastore.Group{
		UID:  uuid.NewString(),
		Name: "Random Group",
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))

	app := &datastore.Application{
		Title:   "Application 10",
		GroupID: newGroup.UID,
		UID:     uuid.NewString(),
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))
}
