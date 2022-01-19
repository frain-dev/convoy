//go:build integration
// +build integration

package mongo

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func Test_FetchGroupByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)

	newOrg := &datastore.Group{
		Name: "Yet another group",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	// Fetch org again
	org, err := groupRepo.FetchGroupByID(context.Background(), newOrg.UID)
	require.NoError(t, err)

	require.Equal(t, org.UID, newOrg.UID)
}

func Test_CreateGroup(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)

	newOrg := &datastore.Group{
		Name: "Next group",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	// Fetch org again
	org, err := groupRepo.FetchGroupByID(context.Background(), newOrg.UID)
	require.NoError(t, err)

	require.Equal(t, org.UID, newOrg.UID)
}

func Test_CannotCreateGroupWithExistingName(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)

	org := &datastore.Group{
		Name:           "Next group",
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), org))

	org = &datastore.Group{
		Name:           "Next group",
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	require.Error(t, groupRepo.CreateGroup(context.Background(), org))
}

func Test_CanCreateGroupWithExistingNameThatHasBeenDeleted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)

	newOrg := &datastore.Group{
		Name:           "Existing group",
		UID:            uuid.NewString(),
		DeletedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.DeletedDocumentStatus,
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	newOrg = &datastore.Group{
		Name:           "Existing group",
		UID:            uuid.NewString(),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))
}

func Test_LoadGroups(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewGroupRepo(db)

	orgs, err := orgRepo.LoadGroups(context.Background(), &datastore.GroupFilter{})
	require.NoError(t, err)

	require.True(t, len(orgs) > 0)
}
