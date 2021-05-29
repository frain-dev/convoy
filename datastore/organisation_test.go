// +build integration

package datastore

import (
	"context"
	"testing"

	"github.com/hookcamp/hookcamp"
	"github.com/stretchr/testify/require"
)

func Test_FetchOrganisationByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrganisationRepo(db)

	newOrg := &hookcamp.Organisation{
		OrgName: "Yet another organisation",
	}

	require.NoError(t, orgRepo.CreateOrganisation(context.Background(), newOrg))

	// Fetch org again
	org, err := orgRepo.FetchOrganisationByID(context.Background(), newOrg.UID)
	require.NoError(t, err)

	require.Equal(t, org.UID, newOrg.UID)
}

func Test_CreateOrganisation(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrganisationRepo(db)

	newOrg := &hookcamp.Organisation{
		OrgName: "Next organisation",
	}

	require.NoError(t, orgRepo.CreateOrganisation(context.Background(), newOrg))

	// Fetch org again
	org, err := orgRepo.FetchOrganisationByID(context.Background(), newOrg.UID)
	require.NoError(t, err)

	require.Equal(t, org.UID, newOrg.UID)
}

func Test_LoadOrganisations(t *testing.T) {
	t.Skip()
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrganisationRepo(db)

	orgs, err := orgRepo.LoadOrganisations(context.Background())
	require.NoError(t, err)

	require.True(t, len(orgs) > 0)
}
