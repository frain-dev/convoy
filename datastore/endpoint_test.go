// +build integration

package datastore

import (
	"context"
	"testing"

	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/util"
	"github.com/stretchr/testify/require"
)

func Test_CreateEndpoint(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrganisationRepo(db)
	appRepo := NewApplicationRepo(db)
	endpointRepo := NewEndpointRepository(db)

	newOrg := &hookcamp.Organisation{
		OrgName: "Random new organisation",
	}

	require.NoError(t, orgRepo.CreateOrganisation(context.Background(), newOrg))

	app := &hookcamp.Application{
		Title: "Next application name",
		OrgID: newOrg.ID,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))

	secret, err := util.GenerateRandomString(20)
	require.NoError(t, err)

	e := &hookcamp.Endpoint{
		AppID:       app.ID,
		Description: "Yet another random endpoint",
		Secret:      secret,
		TargetURL:   "https://google.com",
	}

	require.NoError(t, endpointRepo.CreateEndpoint(context.Background(), e))
}

func Test_FindEndpointByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrganisationRepo(db)
	appRepo := NewApplicationRepo(db)
	endpointRepo := NewEndpointRepository(db)

	newOrg := &hookcamp.Organisation{
		OrgName: "Random new organisation",
	}

	require.NoError(t, orgRepo.CreateOrganisation(context.Background(), newOrg))

	app := &hookcamp.Application{
		Title: "Next application name",
		OrgID: newOrg.ID,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))

	secret, err := util.GenerateRandomString(20)
	require.NoError(t, err)

	e := &hookcamp.Endpoint{
		AppID:       app.ID,
		Description: "Yet another random endpoint",
		Secret:      secret,
		TargetURL:   "https://google.com",
	}

	require.NoError(t, endpointRepo.CreateEndpoint(context.Background(), e))

	_, err = endpointRepo.FindEndpointByID(context.Background(), e.ID)
	require.NoError(t, err)
}
