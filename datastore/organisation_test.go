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

func Test_FetchOrganisationByID(t *testing.T) {

	// See testdata/organisations.yml
	id := uuid.MustParse("2dade341-799e-4bb7-bf4a-b04a23b551c3")

	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrganisationRepo(db)

	org, err := orgRepo.FetchOrganisationByID(context.Background(), id)
	require.NoError(t, err)

	require.Equal(t, org.ID, id)

	_, err = orgRepo.FetchOrganisationByID(context.Background(), uuid.New())
	require.Error(t, err)

	require.True(t, errors.Is(err, hookcamp.ErrOrganisationNotFound))
}
