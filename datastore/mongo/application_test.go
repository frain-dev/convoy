//go:build integration
// +build integration

package mongo

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_UpdateApplication(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(getStore(db))
	appRepo := NewApplicationRepo(getStore(db))

	newGroup := &datastore.Group{
		Name: "Random new group",
		UID:  uuid.NewString(),
	}

	groupCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.GroupCollection)
	require.NoError(t, groupRepo.CreateGroup(groupCtx, newGroup))

	app := &datastore.Application{
		Title:          "Next application name",
		GroupID:        newGroup.UID,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	appCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.AppCollection)
	require.NoError(t, appRepo.CreateApplication(appCtx, app, app.GroupID))

	newTitle := "Newer name"

	app.Title = newTitle

	require.NoError(t, appRepo.UpdateApplication(appCtx, app, app.GroupID))

	newApp, err := appRepo.FindApplicationByID(appCtx, app.UID)
	require.NoError(t, err)

	require.Equal(t, newTitle, newApp.Title)

	app2 := &datastore.Application{
		Title:          newTitle,
		GroupID:        newGroup.UID,
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err = appRepo.CreateApplication(appCtx, app2, app2.GroupID)
	require.Equal(t, datastore.ErrDuplicateAppName, err)
}

func Test_CreateApplication(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(getStore(db))
	appRepo := NewApplicationRepo(getStore(db))

	newOrg := &datastore.Group{
		Name: "Random new group 2",
		UID:  uuid.NewString(),
	}

	groupCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.GroupCollection)
	require.NoError(t, groupRepo.CreateGroup(groupCtx, newOrg))

	app := &datastore.Application{
		Title:          "Next application name",
		GroupID:        newOrg.UID,
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	appCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.AppCollection)
	require.NoError(t, appRepo.CreateApplication(appCtx, app, app.GroupID))

	app2 := &datastore.Application{
		Title:          "Next application name",
		GroupID:        newOrg.UID,
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err := appRepo.CreateApplication(appCtx, app2, app2.GroupID)
	require.Equal(t, datastore.ErrDuplicateAppName, err)
}

func Test_LoadApplicationsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(getStore(db))

	appCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.AppCollection)
	apps, _, err := appRepo.LoadApplicationsPaged(appCtx, "", "", datastore.Pageable{
		Page:    1,
		PerPage: 10,
	})
	require.NoError(t, err)

	require.True(t, len(apps) == 0)
}

func Test_FindApplicationByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(getStore(db))

	appCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.AppCollection)
	_, err := appRepo.FindApplicationByID(appCtx, uuid.New().String())
	require.Error(t, err)

	require.True(t, errors.Is(err, datastore.ErrApplicationNotFound))

	groupRepo := NewGroupRepo(getStore(db))

	newGroup := &datastore.Group{
		Name: "Yet another Random new group",
	}

	groupCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.GroupCollection)
	require.NoError(t, groupRepo.CreateGroup(groupCtx, newGroup))

	app := &datastore.Application{
		Title:   "Next application name again",
		GroupID: newGroup.UID,
		UID:     uuid.NewString(),
	}

	require.NoError(t, appRepo.CreateApplication(appCtx, app, app.GroupID))
}
