// +build integration

package datastore

import (
	"context"
	"testing"

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
