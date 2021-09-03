// +build integration

package datastore

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"github.com/stretchr/testify/require"
)

func Test_UpdateApplication(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrganisationRepo(db)
	appRepo := NewApplicationRepo(db)

	newOrg := &hookcamp.Organisation{
		OrgName: "Random new organisation",
	}

	require.NoError(t, orgRepo.CreateOrganisation(context.Background(), newOrg))

	app := &hookcamp.Application{
		Title: "Next application name",
		OrgID: newOrg.UID,
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

	orgRepo := NewOrganisationRepo(db)
	appRepo := NewApplicationRepo(db)

	newOrg := &hookcamp.Organisation{
		OrgName: "Random new organisation",
	}

	require.NoError(t, orgRepo.CreateOrganisation(context.Background(), newOrg))

	app := &hookcamp.Application{
		Title: "Next application name",
		OrgID: newOrg.UID,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))
}

func Test_LoadApplications(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(db)

	apps, err := appRepo.LoadApplications(context.Background(), "")
	require.NoError(t, err)

	require.True(t, len(apps) > 0)
}

func Test_FindApplicationByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(db)

	app, err := appRepo.FindApplicationByID(context.Background(), uuid.New().String())
	require.Error(t, err)

	require.True(t, errors.Is(err, hookcamp.ErrApplicationNotFound))

	orgRepo := NewOrganisationRepo(db)

	newOrg := &hookcamp.Organisation{
		OrgName: "Yet another Random new organisation",
	}

	require.NoError(t, orgRepo.CreateOrganisation(context.Background(), newOrg))

	app = &hookcamp.Application{
		Title: "Next application name again",
		OrgID: newOrg.UID,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))
}
