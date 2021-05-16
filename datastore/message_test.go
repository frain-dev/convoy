// +build integration

package datastore

import (
	"context"
	"testing"

	"github.com/hookcamp/hookcamp"
	"github.com/stretchr/testify/require"
)

func Test_CreateMessage(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrganisationRepo(db)
	appRepo := NewApplicationRepo(db)
	messageRepo := NewMessageRepository(db)

	newOrg := &hookcamp.Organisation{
		OrgName: "Random new organisation",
	}

	require.NoError(t, orgRepo.CreateOrganisation(context.Background(), newOrg))

	app := &hookcamp.Application{
		Title: "Next application name",
		OrgID: newOrg.ID,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))

	msg := &hookcamp.Message{
		AppID:    app.ID,
		Data:     hookcamp.JSONData([]byte(`{"oops" : "oops"}`)),
		Metadata: &hookcamp.MessageMetadata{},
	}

	require.NoError(t, messageRepo.CreateMessage(context.Background(), msg))
}
