//go:build integration
// +build integration

package mongo

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

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
		Title:          "Next application name",
		GroupID:        newGroup.UID,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))

	newTitle := "Newer name"

	app.Title = newTitle

	require.NoError(t, appRepo.UpdateApplication(context.Background(), app))

	newApp, err := appRepo.FindApplicationByID(context.Background(), app.UID)
	require.NoError(t, err)

	require.Equal(t, newTitle, newApp.Title)
}

func Test_CreateApplication(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	newOrg := &datastore.Group{
		Name: "Random new group 2",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	app := &datastore.Application{
		Title:          "Next application name",
		GroupID:        newOrg.UID,
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))

	app2 := &datastore.Application{
		Title:          "Next application name",
		GroupID:        newOrg.UID,
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err := appRepo.CreateApplication(context.Background(), app2)
	require.Error(t, err)
	require.True(t, true, mongo.IsDuplicateKeyError(err))
	require.Contains(t, err.Error(), "title")
}

func Test_IsAppTitleUnique(t *testing.T) {
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
		Title:          "Application_1",
		GroupID:        newOrg.UID,
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))

	unique, err := appRepo.IsAppTitleUnique(context.Background(), app.Title, newOrg.UID)
	require.NoError(t, err)
	require.Equal(t, false, unique)

	unique, err = appRepo.IsAppTitleUnique(context.Background(), "123", newOrg.UID)
	require.NoError(t, err)
	require.Equal(t, true, unique)
}

func Test_LoadApplicationsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(db)

	apps, _, err := appRepo.LoadApplicationsPaged(context.Background(), "", "", datastore.Pageable{
		Page:    1,
		PerPage: 10,
	})
	require.NoError(t, err)

	require.True(t, len(apps) > 0)
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
		Name: "Yet another Random new group",
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))

	app := &datastore.Application{
		Title:   "Next application name again",
		GroupID: newGroup.UID,
		UID:     uuid.NewString(),
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))
}
