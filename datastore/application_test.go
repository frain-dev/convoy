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
		OrgID: newOrg.ID,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))
}

func Test_LoadApplications(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(db)

	apps, err := appRepo.LoadApplications(context.Background())
	require.NoError(t, err)

	require.True(t, len(apps) > 0)
}

func Test_FindApplicationByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(db)

	app, err := appRepo.FindApplicationByID(context.Background(), uuid.New())
	require.Error(t, err)

	require.True(t, errors.Is(err, hookcamp.ErrApplicationNotFound))

	// look at testdata/applications.yml
	app, err = appRepo.FindApplicationByID(context.Background(), uuid.MustParse("f98f8de6-a972-4609-88e6-61cd7ecf4e3a"))
	require.NoError(t, err)

	require.Equal(t, app.ID.String(), "f98f8de6-a972-4609-88e6-61cd7ecf4e3a")
}
