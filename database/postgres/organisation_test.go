//go:build integration
// +build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func TestLoadOrganisationsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrgRepo(db)

	user := seedUser(t, db)

	for i := 1; i < 6; i++ {
		org := &datastore.Organisation{
			UID:            ulid.Make().String(),
			OwnerID:        user.UID,
			Name:           fmt.Sprintf("org%d", i),
			CustomDomain:   null.NewString(ulid.Make().String(), true),
			AssignedDomain: null.NewString(ulid.Make().String(), true),
		}

		err := orgRepo.CreateOrganisation(context.Background(), org)
		require.NoError(t, err)
	}

	organisations, _, err := orgRepo.LoadOrganisationsPaged(context.Background(), datastore.Pageable{
		Page:    2,
		PerPage: 2,
		Sort:    -1,
	})

	require.NoError(t, err)
	require.Equal(t, 2, len(organisations))
}

func TestCreateOrganisation(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	user := seedUser(t, db)

	org := &datastore.Organisation{
		UID:            ulid.Make().String(),
		Name:           "new org",
		OwnerID:        user.UID,
		CustomDomain:   null.NewString("https://google.com", true),
		AssignedDomain: null.NewString("https://google.com", true),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := NewOrgRepo(db).CreateOrganisation(context.Background(), org)
	require.NoError(t, err)
}

func TestUpdateOrganisation(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrgRepo(db)

	user := seedUser(t, db)

	org := &datastore.Organisation{
		Name:           "new org",
		OwnerID:        user.UID,
		CustomDomain:   null.NewString(ulid.Make().String(), true),
		AssignedDomain: null.NewString(ulid.Make().String(), true),
	}

	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	name := "organisation update"
	org.Name = name
	newDomain := null.NewString("https://yt.com", true)

	org.CustomDomain = newDomain
	org.AssignedDomain = newDomain

	err = orgRepo.UpdateOrganisation(context.Background(), org)
	require.NoError(t, err)

	dbOrg, err := orgRepo.FetchOrganisationByID(context.Background(), org.UID)
	require.NoError(t, err)

	require.Equal(t, name, dbOrg.Name)
	require.Equal(t, newDomain, dbOrg.CustomDomain)
	require.Equal(t, newDomain, dbOrg.AssignedDomain)
}

func TestFetchOrganisationByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	user := seedUser(t, db)

	orgRepo := NewOrgRepo(db)

	org := &datastore.Organisation{
		UID:            ulid.Make().String(),
		Name:           "new org",
		OwnerID:        user.UID,
		CustomDomain:   null.NewString("https://google.com", true),
		AssignedDomain: null.NewString("https://google.com", true),
	}

	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	dbOrg, err := orgRepo.FetchOrganisationByID(context.Background(), org.UID)
	require.NoError(t, err)
	require.NotEmpty(t, dbOrg.CreatedAt)
	require.NotEmpty(t, dbOrg.UpdatedAt)

	dbOrg.CreatedAt = time.Time{}
	dbOrg.UpdatedAt = time.Time{}

	require.Equal(t, org, dbOrg)
}

func TestFetchOrganisationByAssignedDomain(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	user := seedUser(t, db)

	orgRepo := NewOrgRepo(db)

	org := &datastore.Organisation{
		UID:            ulid.Make().String(),
		Name:           "new org",
		OwnerID:        user.UID,
		CustomDomain:   null.NewString("https://yt.com", true),
		AssignedDomain: null.NewString("https://google.com", true),
	}

	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	dbOrg, err := orgRepo.FetchOrganisationByAssignedDomain(context.Background(), "https://google.com")
	require.NoError(t, err)
	require.NotEmpty(t, dbOrg.CreatedAt)
	require.NotEmpty(t, dbOrg.UpdatedAt)

	dbOrg.CreatedAt = time.Time{}
	dbOrg.UpdatedAt = time.Time{}

	require.Equal(t, org, dbOrg)
}

func TestFetchOrganisationByCustomDomain(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	user := seedUser(t, db)

	orgRepo := NewOrgRepo(db)

	org := &datastore.Organisation{
		UID:            ulid.Make().String(),
		Name:           "new org",
		OwnerID:        user.UID,
		CustomDomain:   null.NewString("https://yt.com", true),
		AssignedDomain: null.NewString("https://google.com", true),
	}

	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	dbOrg, err := orgRepo.FetchOrganisationByCustomDomain(context.Background(), "https://yt.com")
	require.NoError(t, err)
	require.NotEmpty(t, dbOrg.CreatedAt)
	require.NotEmpty(t, dbOrg.UpdatedAt)

	dbOrg.CreatedAt = time.Time{}
	dbOrg.UpdatedAt = time.Time{}

	require.Equal(t, org, dbOrg)
}

func TestDeleteOrganisation(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewOrgRepo(db)
	user := seedUser(t, db)

	org := &datastore.Organisation{Name: "new org", OwnerID: user.UID}

	err := orgRepo.CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	err = orgRepo.DeleteOrganisation(context.Background(), org.UID)
	require.NoError(t, err)

	_, err = orgRepo.FetchOrganisationByID(context.Background(), org.UID)
	require.Equal(t, datastore.ErrOrgNotFound, err)
}

func seedUser(t *testing.T, db database.Database) *datastore.User {
	user := generateUser(t)

	err := NewUserRepo(db).CreateUser(context.Background(), user)
	require.NoError(t, err)

	return user
}
