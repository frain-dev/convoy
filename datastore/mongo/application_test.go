//go:build integration
// +build integration

package mongo

import (
	"context"
	"errors"
	"fmt"
	"testing"

	log "github.com/sirupsen/logrus"

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

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))

	app := &datastore.Application{
		Title:     "Next application name",
		UID:       "app_id",
		DeletedAt: nil,
		GroupID:   newGroup.UID,
	}

	err := appRepo.CreateApplication(context.Background(), app, app.GroupID)
	require.NoError(t, err)
	log.Fatal("dd")
	newTitle := "Newer name"
	app.Title = newTitle

	require.NoError(t, appRepo.UpdateApplication(context.Background(), app, app.GroupID))

	newApp, err := appRepo.FindApplicationByID(context.Background(), app.UID)
	require.NoError(t, err)
	fmt.Printf("tt %+v", newApp)
	require.Equal(t, newTitle, newApp.Title)

	app2 := &datastore.Application{
		Title:   newTitle,
		GroupID: newGroup.UID,
		UID:     uuid.NewString(),
	}

	err = appRepo.CreateApplication(context.Background(), app2, app2.GroupID)
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

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	app := &datastore.Application{
		Title:   "Next application name",
		GroupID: newOrg.UID,
		UID:     uuid.NewString(),
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app, app.GroupID))

	app2 := &datastore.Application{
		Title:   "Next application name",
		GroupID: newOrg.UID,
		UID:     uuid.NewString(),
	}

	err := appRepo.CreateApplication(context.Background(), app2, app2.GroupID)
	require.Equal(t, datastore.ErrDuplicateAppName, err)
}

func Test_LoadApplicationsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(getStore(db))

	apps, _, err := appRepo.LoadApplicationsPaged(context.Background(), "", "", datastore.Pageable{
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

	_, err := appRepo.FindApplicationByID(context.Background(), uuid.New().String())
	require.Error(t, err)

	require.True(t, errors.Is(err, datastore.ErrApplicationNotFound))

	groupRepo := NewGroupRepo(getStore(db))

	newGroup := &datastore.Group{
		Name: "Yet another Random new group",
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))

	app := &datastore.Application{
		Title:   "Next application name again",
		GroupID: newGroup.UID,
		UID:     uuid.NewString(),
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app, app.GroupID))
}
