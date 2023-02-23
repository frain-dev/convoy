package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func Test_CreateAPIKey(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey()

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	newApiKey, err := apiKeyRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.NoError(t, err)

	apiKey.ExpiresAt = time.Time{}
	newApiKey.CreatedAt = time.Time{}
	newApiKey.UpdatedAt = time.Time{}
	newApiKey.ExpiresAt = time.Time{}

	require.Equal(t, apiKey, newApiKey)
}

func Test_FindAPIKeyByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey()

	_, err := apiKeyRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrAPIKeyNotFound))

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	newApiKey, err := apiKeyRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.NoError(t, err)

	apiKey.ExpiresAt = time.Time{}
	newApiKey.CreatedAt = time.Time{}
	newApiKey.UpdatedAt = time.Time{}
	newApiKey.ExpiresAt = time.Time{}

	require.Equal(t, apiKey, newApiKey)
}

func Test_FindAPIKeyByMaskID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey()

	_, err := apiKeyRepo.FindAPIKeyByMaskID(context.Background(), apiKey.MaskID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrAPIKeyNotFound))

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	newApiKey, err := apiKeyRepo.FindAPIKeyByMaskID(context.Background(), apiKey.MaskID)
	require.NoError(t, err)

	apiKey.ExpiresAt = time.Time{}
	newApiKey.CreatedAt = time.Time{}
	newApiKey.UpdatedAt = time.Time{}
	newApiKey.ExpiresAt = time.Time{}

	require.Equal(t, apiKey, newApiKey)
}

func Test_FindAPIKeyByHash(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey()

	_, err := apiKeyRepo.FindAPIKeyByHash(context.Background(), apiKey.Hash)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrAPIKeyNotFound))

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	newApiKey, err := apiKeyRepo.FindAPIKeyByHash(context.Background(), apiKey.Hash)
	require.NoError(t, err)

	apiKey.ExpiresAt = time.Time{}
	newApiKey.CreatedAt = time.Time{}
	newApiKey.UpdatedAt = time.Time{}
	newApiKey.ExpiresAt = time.Time{}

	require.Equal(t, apiKey, newApiKey)
}

func Test_UpdateAPIKey(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey()

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))

	apiKey.Name = "Updated-Test-Api-Key"
	apiKey.Role = auth.Role{
		Type: auth.RoleSuperUser,
	}

	require.NoError(t, apiKeyRepo.UpdateAPIKey(context.Background(), apiKey))

	newApiKey, err := apiKeyRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.NoError(t, err)

	apiKey.ExpiresAt = time.Time{}
	newApiKey.CreatedAt = time.Time{}
	newApiKey.UpdatedAt = time.Time{}
	newApiKey.ExpiresAt = time.Time{}

	require.Equal(t, apiKey, newApiKey)
}

func Test_RevokeAPIKey(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewAPIKeyRepo(db)
	apiKey := generateApiKey()

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
			pageData: datastore.Pageable{Page: 1, PerPage: 3},
			count:    10,
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     10,
					TotalPage: 4,
					Page:      1,
					PerPage:   3,
					Prev:      1,
					Next:      2,
				},
			},
		},

		{
			name:     "Load API Keys Paged - 12 records",
			pageData: datastore.Pageable{Page: 2, PerPage: 4},
			count:    12,
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     12,
					TotalPage: 3,
					Page:      2,
					PerPage:   4,
					Prev:      1,
					Next:      3,
				},
			},
		},

		{
			name:     "Load API Keys Paged - 5 records",
			pageData: datastore.Pageable{Page: 1, PerPage: 3},
			count:    5,
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     5,
					TotalPage: 2,
					Page:      1,
					PerPage:   3,
					Prev:      1,
					Next:      2,
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
						Type:     auth.RoleAdmin,
						Project:  project.UID,
						Endpoint: ulid.Make().String(),
					},
					Hash:      ulid.Make().String(),
					Salt:      ulid.Make().String(),
					ExpiresAt: time.Now().Add(5 * time.Minute),
				}
				require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), apiKey))
			}

			_, pageable, err := apiKeyRepo.LoadAPIKeysPaged(context.Background(), &datastore.ApiKeyFilter{ProjectID: project.UID}, &tc.pageData)

			require.NoError(t, err)

			require.Equal(t, tc.expected.paginationData.Total, pageable.Total)
			require.Equal(t, tc.expected.paginationData.TotalPage, pageable.TotalPage)
			require.Equal(t, tc.expected.paginationData.Page, pageable.Page)
			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
			require.Equal(t, tc.expected.paginationData.Prev, pageable.Prev)
			require.Equal(t, tc.expected.paginationData.Next, pageable.Next)
		})
	}
}

func generateApiKey() *datastore.APIKey {
	return &datastore.APIKey{
		UID:    ulid.Make().String(),
		MaskID: ulid.Make().String(),
		Name:   "Test Api Key",
		Type:   datastore.ProjectKey,
		Role: auth.Role{
			Type:     auth.RoleAdmin,
			Project:  ulid.Make().String(),
			Endpoint: ulid.Make().String(),
		},
		Hash:      ulid.Make().String(),
		Salt:      ulid.Make().String(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
}
