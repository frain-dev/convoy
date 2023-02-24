//go:build integration
// +build integration

package mongo

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func Test_UpdateEndpoint(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	projectRepo := NewProjectRepo(getStore(db))
	endpointRepo := NewEndpointRepo(getStore(db))

	newProject := &datastore.Project{
		Name: "Random new project",
		UID:  ulid.Make().String(),
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), newProject))

	endpoint := &datastore.Endpoint{
		Title:     "Next application name",
		ProjectID: newProject.UID,
	}

	require.NoError(t, endpointRepo.CreateEndpoint(context.Background(), endpoint, endpoint.ProjectID))

	newTitle := "Newer name"

	endpoint.Title = newTitle

	require.NoError(t, endpointRepo.UpdateEndpoint(context.Background(), endpoint, endpoint.ProjectID))

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
		Name: "Random new project 2",
		UID:  ulid.Make().String(),
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), newOrg))

	endpoint := &datastore.Endpoint{
		Title:     "Next application name",
		ProjectID: newOrg.UID,
		UID:       ulid.Make().String(),
	}

	require.NoError(t, endpointRepo.CreateEndpoint(context.Background(), endpoint, endpoint.ProjectID))
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

	_, err := endpointRepo.FindEndpointByID(context.Background(), ulid.Make().String())
	require.Error(t, err)

	require.True(t, errors.Is(err, datastore.ErrEndpointNotFound))

	projectRepo := NewProjectRepo(getStore(db))

	newProject := &datastore.Project{
		Name: "Yet another Random new project",
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), newProject))

	endpoint := &datastore.Endpoint{
		Title:     "Next endpoint name again",
		ProjectID: newProject.UID,
		UID:       ulid.Make().String(),
	}

	require.NoError(t, endpointRepo.CreateEndpoint(context.Background(), endpoint, endpoint.ProjectID))
}
