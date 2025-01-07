//go:build integration
// +build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func Test_CreateAPIKey(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	project := seedProject(t, db)
	endpoint := seedEndpoint(t, db)

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey(project, endpoint)

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	newApiKey, err := apiKeyRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.NoError(t, err)

	apiKey.ExpiresAt = null.Time{}
	newApiKey.CreatedAt = time.Time{}
	newApiKey.UpdatedAt = time.Time{}
	newApiKey.ExpiresAt = null.Time{}

	require.Equal(t, apiKey, newApiKey)
}

func Test_FindAPIKeyByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	project := seedProject(t, db)
	endpoint := seedEndpoint(t, db)

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey(project, endpoint)

	_, err := apiKeyRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrAPIKeyNotFound))

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	newApiKey, err := apiKeyRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.NoError(t, err)

	apiKey.ExpiresAt = null.Time{}
	newApiKey.CreatedAt = time.Time{}
	newApiKey.UpdatedAt = time.Time{}
	newApiKey.ExpiresAt = null.Time{}

	require.Equal(t, apiKey, newApiKey)
}

func Test_FindAPIKeyByMaskID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	project := seedProject(t, db)
	endpoint := seedEndpoint(t, db)

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey(project, endpoint)

	_, err := apiKeyRepo.FindAPIKeyByMaskID(context.Background(), apiKey.MaskID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrAPIKeyNotFound))

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	newApiKey, err := apiKeyRepo.FindAPIKeyByMaskID(context.Background(), apiKey.MaskID)
	require.NoError(t, err)

	apiKey.ExpiresAt = null.Time{}
	newApiKey.CreatedAt = time.Time{}
	newApiKey.UpdatedAt = time.Time{}
	newApiKey.ExpiresAt = null.Time{}

	require.Equal(t, apiKey, newApiKey)
}

func Test_FindAPIKeyByHash(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	project := seedProject(t, db)
	endpoint := seedEndpoint(t, db)

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey(project, endpoint)

	_, err := apiKeyRepo.FindAPIKeyByHash(context.Background(), apiKey.Hash)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrAPIKeyNotFound))

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	newApiKey, err := apiKeyRepo.FindAPIKeyByHash(context.Background(), apiKey.Hash)
	require.NoError(t, err)

	apiKey.ExpiresAt = null.Time{}
	newApiKey.CreatedAt = time.Time{}
	newApiKey.UpdatedAt = time.Time{}
	newApiKey.ExpiresAt = null.Time{}

	require.Equal(t, apiKey, newApiKey)
}

func Test_UpdateAPIKey(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	project := seedProject(t, db)
	endpoint := seedEndpoint(t, db)

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey(project, endpoint)

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	apiKey.Name = "Updated-Test-Api-Key"
	apiKey.Role = auth.Role{
		Type:    auth.RoleOrganisationAdmin,
		Project: project.UID,
	}

	require.NoError(t, apiKeyRepo.UpdateAPIKey(context.Background(), apiKey))

	newApiKey, err := apiKeyRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.NoError(t, err)

	apiKey.ExpiresAt = null.Time{}
	newApiKey.CreatedAt = time.Time{}
	newApiKey.UpdatedAt = time.Time{}
	newApiKey.ExpiresAt = null.Time{}

	require.Equal(t, apiKey, newApiKey)
}

func Test_RevokeAPIKey(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	project := seedProject(t, db)
	endpoint := seedEndpoint(t, db)

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey(project, endpoint)

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	_, err := apiKeyRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.NoError(t, err)

	require.NoError(t, apiKeyRepo.RevokeAPIKeys(context.Background(), []string{apiKey.UID}))

	_, err = apiKeyRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrAPIKeyNotFound))
}

func Test_LoadAPIKeysPaged(t *testing.T) {
	type Expected struct {
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name     string
		pageData datastore.Pageable
		count    int
		expected Expected
	}{
		{
			name:     "Load API Keys Paged - 10 records",
			pageData: datastore.Pageable{PerPage: 3},
			count:    10,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},

		{
			name:     "Load API Keys Paged - 12 records",
			pageData: datastore.Pageable{PerPage: 4},
			count:    12,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 4,
				},
			},
		},

		{
			name:     "Load API Keys Paged - 5 records",
			pageData: datastore.Pageable{PerPage: 3},
			count:    5,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			project := seedProject(t, db)

			apiKeyRepo := NewAPIKeyRepo(db)
			for i := 0; i < tc.count; i++ {
				apiKey := &datastore.APIKey{
					UID:    ulid.Make().String(),
					MaskID: ulid.Make().String(),
					Name:   "Test Api Key",
					Type:   datastore.ProjectKey,
					Role: auth.Role{
						Type:    auth.RoleAdmin,
						Project: project.UID,
					},
					Hash:      ulid.Make().String(),
					Salt:      ulid.Make().String(),
					ExpiresAt: null.NewTime(time.Now().Add(5*time.Minute), true),
				}
				require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))
			}

			_, pageable, err := apiKeyRepo.LoadAPIKeysPaged(context.Background(), &datastore.ApiKeyFilter{ProjectID: project.UID}, &tc.pageData)

			require.NoError(t, err)

			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
		})
	}
}

func generateApiKey(project *datastore.Project, endpoint *datastore.Endpoint) *datastore.APIKey {
	return &datastore.APIKey{
		UID:    ulid.Make().String(),
		MaskID: ulid.Make().String(),
		Name:   "Test Api Key",
		Type:   datastore.ProjectKey,
		Role: auth.Role{
			Type:     auth.RoleAdmin,
			Project:  project.UID,
			Endpoint: endpoint.UID,
		},
		Hash:      ulid.Make().String(),
		Salt:      ulid.Make().String(),
		ExpiresAt: null.NewTime(time.Now().Add(5*time.Minute), true),
	}
}
