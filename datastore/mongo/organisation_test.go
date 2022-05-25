//go:build integration
// +build integration

package mongo

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
	"time"
)

func TestLoadOrganisationsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrgRepo(db)

	for i := 1; i < 6; i++ {
		org := &datastore.Organisation{
			UID:            uuid.NewString(),
			Name:           fmt.Sprintf("org%d", i),
			DocumentStatus: datastore.ActiveDocumentStatus,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		}

		err := orgRepo.CreateOrganisation(context.Background(), org)
		require.NoError(t, err)
	}

	organisations, _, err := orgRepo.LoadOrganisationsPaged(context.Background(), datastore.Pageable{
		Page:    2,
		PerPage: 2,
	})

	require.NoError(t, err)
	require.Equal(t, 2, len(organisations))
	require.Equal(t, "org3", organisations[0].Name)
	require.Equal(t, "org2", organisations[1].Name)
}

func TestCreateOrganisation(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrgRepo(db)
	org := &datastore.Organisation{
		UID:       uuid.NewString(),
		Name:      fmt.Sprintf("new org"),
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)
}

func TestUpdateOrganisation(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrgRepo(db)
	org := &datastore.Organisation{
		UID:       uuid.NewString(),
		Name:      fmt.Sprintf("new org"),
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	name := "organisation update"
	org.Name = name

	err = orgRepo.UpdateOrganisation(context.Background(), org)
	require.NoError(t, err)

	org, err = orgRepo.FetchOrganisationByID(context.Background(), org.UID)
	require.NoError(t, err)

	require.Equal(t, name, org.Name)
}

func TestFetchOrganisationByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrgRepo(db)
	org := &datastore.Organisation{
		UID:       uuid.NewString(),
		Name:      fmt.Sprintf("new org"),
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	organisation, err := orgRepo.FetchOrganisationByID(context.Background(), org.UID)
	require.NoError(t, err)

	require.Equal(t, org.UID, organisation.UID)
}

func TestDeleteOrganisation(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrgRepo(db)
	org := &datastore.Organisation{
		UID:       uuid.NewString(),
		Name:      fmt.Sprintf("new org"),
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	err = orgRepo.DeleteOrganisation(context.Background(), org.UID)
	require.NoError(t, err)

	organisation, err := orgRepo.FetchOrganisationByID(context.Background(), org.UID)
	require.NoError(t, err)

	require.True(t, organisation.DeletedAt > 0)
	require.Equal(t, datastore.DeletedDocumentStatus, org.DocumentStatus)
}
