//go:build integration
// +build integration

package datastore

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/server/models"

	"github.com/frain-dev/convoy"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_UpdateApplication(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	newGroup := &convoy.Group{
		Name: "Random new group",
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))

	app := &convoy.Application{
		Title:          "Next application name",
		GroupID:        newGroup.UID,
		DocumentStatus: convoy.ActiveDocumentStatus,
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

	newOrg := &convoy.Group{
		Name: "Random new group",
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	app := &convoy.Application{
		Title:   "Next application name",
		GroupID: newOrg.UID,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))
}

func Test_LoadApplicationsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(db)

	apps, _, err := appRepo.LoadApplicationsPaged(context.Background(), "", models.Pageable{
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

	app, err := appRepo.FindApplicationByID(context.Background(), uuid.New().String())
	require.Error(t, err)

	require.True(t, errors.Is(err, convoy.ErrApplicationNotFound))

	groupRepo := NewGroupRepo(db)

	newGroup := &convoy.Group{
		Name: "Yet another Random new group",
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))

	app = &convoy.Application{
		Title:   "Next application name again",
		GroupID: newGroup.UID,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))
}
