//go:build integration
// +build integration

package bolt

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/pbkdf2"
)

func Test_CreateAPIKey(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewApiRoleRepo(db)

	maskID, salt, encodedKey, err := generateAPIKey()

	if err != nil {
		t.Fatal(err)
	}

	role := auth.Role{Type: "super_user"}

	newApiKey := &datastore.APIKey{
		UID:    uuid.New().String(),
		MaskID: maskID,
		Name:   "Api Key",
		Type:   "test_api_key",
		Role:   role,
		Hash:   encodedKey,
		Salt:   salt,
	}

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), newApiKey))
	apiKey, err := apiKeyRepo.FindAPIKeyByID(context.Background(), newApiKey.UID)

	require.NoError(t, err)
	require.Equal(t, newApiKey.UID, apiKey.UID)

}

func Test_UpdateAPIKey(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewApiRoleRepo(db)

	maskID, salt, encodedKey, err := generateAPIKey()

	if err != nil {
		t.Fatal(err)
	}

	role := auth.Role{Type: "super_user"}

	newApiKey := &datastore.APIKey{
		UID:    uuid.New().String(),
		MaskID: maskID,
		Name:   "Api Key",
		Type:   "test_api_key",
		Role:   role,
		Hash:   encodedKey,
		Salt:   salt,
	}

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), newApiKey))

	updatedTitle := "Updated Api Key"
	_, _, encodedKey, err = generateAPIKey()

	if err != nil {
		t.Fatal(err)
	}

	newApiKey.Name = updatedTitle
	newApiKey.Hash = encodedKey

	require.NoError(t, apiKeyRepo.UpdateAPIKey(context.Background(), newApiKey))
	apiKey, err := apiKeyRepo.FindAPIKeyByID(context.Background(), newApiKey.UID)

	require.NoError(t, err)
	require.Equal(t, updatedTitle, apiKey.Name)
	require.Equal(t, encodedKey, apiKey.Hash)
}

func Test_FindAPIKeyByMaskId(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewApiRoleRepo(db)

	maskID, salt, encodedKey, err := generateAPIKey()

	if err != nil {
		t.Fatal(err)
	}

	role := auth.Role{Type: "super_user"}

	newApiKey := &datastore.APIKey{
		UID:    uuid.New().String(),
		MaskID: maskID,
		Name:   "Api Key",
		Type:   "test_api_key",
		Role:   role,
		Hash:   encodedKey,
		Salt:   salt,
	}

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), newApiKey))
	apiKey, err := apiKeyRepo.FindAPIKeyByMaskID(context.Background(), newApiKey.MaskID)

	require.NoError(t, err)
	require.Equal(t, newApiKey.UID, apiKey.UID)
	require.Equal(t, newApiKey.MaskID, apiKey.MaskID)

	//Find Non Existent Mask ID
	apiKey, err = apiKeyRepo.FindAPIKeyByMaskID(context.Background(), uuid.NewString())
	require.ErrorIs(t, err, datastore.ErrAPIKeyNotFound)
}

func Test_FindAPIKeyByHash(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewApiRoleRepo(db)

	maskID, salt, encodedKey, err := generateAPIKey()

	if err != nil {
		t.Fatal(err)
	}

	role := auth.Role{Type: "super_user"}

	newApiKey := &datastore.APIKey{
		UID:    uuid.New().String(),
		MaskID: maskID,
		Name:   "Api Key",
		Type:   "test_api_key",
		Role:   role,
		Hash:   encodedKey,
		Salt:   salt,
	}

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), newApiKey))
	apiKey, err := apiKeyRepo.FindAPIKeyByHash(context.Background(), newApiKey.Hash)

	require.NoError(t, err)
	require.Equal(t, newApiKey.UID, apiKey.UID)
	require.Equal(t, newApiKey.Hash, apiKey.Hash)

	//Find Non Existent Hash
	apiKey, err = apiKeyRepo.FindAPIKeyByHash(context.Background(), uuid.NewString())
	require.ErrorIs(t, err, datastore.ErrAPIKeyNotFound)
}

func Test_RevokeAPIKeys(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	apiKeyRepo := NewApiRoleRepo(db)

	maskID, salt, encodedKey, err := generateAPIKey()

	if err != nil {
		t.Fatal(err)
	}

	role := auth.Role{Type: "super_user"}

	newApiKey := &datastore.APIKey{
		UID:    uuid.New().String(),
		MaskID: maskID,
		Name:   "Api Key",
		Type:   "test_api_key",
		Role:   role,
		Hash:   encodedKey,
		Salt:   salt,
	}

	require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), newApiKey))

	ids := []string{newApiKey.UID}
	require.NoError(t, apiKeyRepo.RevokeAPIKeys(context.Background(), ids))

	//Find Deleted API Key
	_, err = apiKeyRepo.FindAPIKeyByID(context.Background(), newApiKey.UID)
	require.ErrorIs(t, err, datastore.ErrAPIKeyNotFound)
}

func Test_LoadAPIKeysPaged(t *testing.T) {
	type ApiKey struct {
		UID   string
		Name  string
		Count int
	}

	type Expected struct {
		ApiKeyCount    int
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name     string
		pageData datastore.Pageable
		apiKeys  []ApiKey
		expected Expected
	}{
		{
			name:     "Load API Keys Paged with 10 records",
			pageData: datastore.Pageable{Page: 1, PerPage: 3},
			apiKeys: []ApiKey{
				{
					UID:   uuid.NewString(),
					Name:  "Group 1",
					Count: 10,
				},
			},
			expected: Expected{
				ApiKeyCount:    3,
				paginationData: datastore.PaginationData{Total: 10, TotalPage: 4, Page: 1, PerPage: 3, Prev: 0, Next: 2},
			},
		},

		{
			name:     "Load API Keys Paged with 13 records",
			pageData: datastore.Pageable{Page: 2, PerPage: 5},
			apiKeys: []ApiKey{
				{
					UID:   uuid.NewString(),
					Name:  "Group 2",
					Count: 13,
				},
			},
			expected: Expected{
				ApiKeyCount:    5,
				paginationData: datastore.PaginationData{Total: 13, TotalPage: 3, Page: 2, PerPage: 5, Prev: 1, Next: 3},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			db, closeFn := getDB(t)
			defer closeFn()

			apiKeyRepo := NewApiRoleRepo(db)

			for _, key := range tc.apiKeys {

				for i := 0; i < key.Count; i++ {
					maskID, salt, encodedKey, err := generateAPIKey()

					if err != nil {
						t.Fatal(err)
					}

					role := auth.Role{Type: "super_user"}

					newApiKey := &datastore.APIKey{
						UID:    uuid.New().String(),
						MaskID: maskID,
						Name:   key.Name,
						Type:   "test_api_key",
						Role:   role,
						Hash:   encodedKey,
						Salt:   salt,
					}

					require.NoError(t, apiKeyRepo.CreateAPIKey(context.Background(), newApiKey))
				}
			}

			apiKeys, data, err := apiKeyRepo.LoadAPIKeysPaged(context.Background(), &tc.pageData)

			require.NoError(t, err)
			require.Equal(t, tc.expected.ApiKeyCount, len(apiKeys))

			require.Equal(t, tc.expected.paginationData.Total, data.Total)
			require.Equal(t, tc.expected.paginationData.TotalPage, data.TotalPage)

			require.Equal(t, tc.expected.paginationData.Next, data.Next)
			require.Equal(t, tc.expected.paginationData.Prev, data.Prev)

			require.Equal(t, tc.expected.paginationData.Page, data.Page)
			require.Equal(t, tc.expected.paginationData.PerPage, data.PerPage)

		})
	}
}

func generateAPIKey() (string, string, string, error) {
	var e string

	maskID, key := util.GenerateAPIKey()
	salt, err := util.GenerateSecret()

	if err != nil {
		return e, e, e, err
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	return maskID, salt, encodedKey, nil

}
