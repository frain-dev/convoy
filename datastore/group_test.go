//go:build integration
// +build integration

package datastore

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/stretchr/testify/require"
)

func Test_FetchGroupByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)

	newOrg := &convoy.Group{
		Name: "Yet another group",
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

	newOrg := &convoy.Group{
		Name: "Next group",
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	// Fetch org again
	org, err := groupRepo.FetchGroupByID(context.Background(), newOrg.UID)
	require.NoError(t, err)

	require.Equal(t, org.UID, newOrg.UID)
}

func Test_LoadGroups(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgRepo := NewGroupRepo(db)

	orgs, err := orgRepo.LoadGroups(context.Background())
	require.NoError(t, err)

	require.True(t, len(orgs) > 0)
}
