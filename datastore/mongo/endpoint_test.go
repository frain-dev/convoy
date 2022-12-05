//go:build integration
// +build integration

package mongo

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_UpdateEndpoint(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	projectRepo := NewProjectRepo(getStore(db))
	endpointRepo := NewEndpointRepo(getStore(db))

	newGroup := &datastore.Project{
		Name: "Random new group",
		UID:  uuid.NewString(),
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), newGroup))

	endpoint := &datastore.Endpoint{
		Title:   "Next application name",
		GroupID: newGroup.UID,
	}

	require.NoError(t, endpointRepo.CreateEndpoint(context.Background(), endpoint, endpoint.GroupID))

	newTitle := "Newer name"

	endpoint.Title = newTitle

	require.NoError(t, endpointRepo.UpdateEndpoint(context.Background(), endpoint, endpoint.GroupID))

	newApp, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID)
	require.NoError(t, err)

	require.Equal(t, newTitle, newApp.Title)
}

func Test_CreateEndpoint(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	projectRepo := NewProjectRepo(getStore(db))
	endpointRepo := NewEndpointRepo(getStore(db))

	newOrg := &datastore.Project{
		Name: "Random new group 2",
		UID:  uuid.NewString(),
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), newOrg))

	endpoint := &datastore.Endpoint{
		Title:   "Next application name",
		GroupID: newOrg.UID,
		UID:     uuid.NewString(),
	}

	require.NoError(t, endpointRepo.CreateEndpoint(context.Background(), endpoint, endpoint.GroupID))
}

func Test_LoadEndpointsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(getStore(db))

	endpoints, _, err := endpointRepo.LoadEndpointsPaged(context.Background(), "", "", datastore.Pageable{
		Page:    1,
		PerPage: 10,
	})
	require.NoError(t, err)

	require.True(t, len(endpoints) == 0)
}

func Test_FindEndpointByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(getStore(db))

	_, err := endpointRepo.FindEndpointByID(context.Background(), uuid.New().String())
	require.Error(t, err)

	require.True(t, errors.Is(err, datastore.ErrEndpointNotFound))

	projectRepo := NewProjectRepo(getStore(db))

	newGroup := &datastore.Project{
		Name: "Yet another Random new group",
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), newGroup))

	endpoint := &datastore.Endpoint{
		Title:   "Next endpoint name again",
		GroupID: newGroup.UID,
		UID:     uuid.NewString(),
	}

	require.NoError(t, endpointRepo.CreateEndpoint(context.Background(), endpoint, endpoint.GroupID))
}
