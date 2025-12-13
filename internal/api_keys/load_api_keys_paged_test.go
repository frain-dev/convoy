package api_keys

import (
	"fmt"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
)

// ============================================================================
// LoadAPIKeysPaged Tests
// ============================================================================

func TestLoadAPIKeysPaged_ForwardFirstPage(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create 15 API keys
	for i := 0; i < 15; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("Key %d", i),
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	// Fetch first page
	filter := &datastore.ApiKeyFilter{
		ProjectID: project.UID,
	}
	pageable := datastore.Pageable{
		PerPage:   5,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	keys, pagination, err := service.LoadAPIKeysPaged(ctx, filter, pageable)

	require.NoError(t, err)
	require.Len(t, keys, 5)
	require.True(t, pagination.HasNextPage)
	require.False(t, pagination.HasPreviousPage)
}

func TestLoadAPIKeysPaged_ForwardSecondPage(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create 15 API keys
	for i := 0; i < 15; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("Key %d", i),
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	// Fetch first page
	pageable1 := datastore.Pageable{
		PerPage:   5,
		Direction: datastore.Next,
	}
	pageable1.SetCursors()
	filter := &datastore.ApiKeyFilter{ProjectID: project.UID}

	keys1, pagination1, err := service.LoadAPIKeysPaged(ctx, filter, pageable1)
	require.NoError(t, err)
	require.Len(t, keys1, 5)

	// Fetch second page
	pageable2 := datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Next,
		NextCursor: pagination1.NextPageCursor,
	}

	keys2, pagination2, err := service.LoadAPIKeysPaged(ctx, filter, pageable2)

	require.NoError(t, err)
	require.Len(t, keys2, 5)
	require.True(t, pagination2.HasNextPage)
	require.True(t, pagination2.HasPreviousPage)

	// Verify no overlap
	for _, k1 := range keys1 {
		for _, k2 := range keys2 {
			require.NotEqual(t, k1.UID, k2.UID)
		}
	}
}

func TestLoadAPIKeysPaged_EmptyResults(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Don't create any keys

	filter := &datastore.ApiKeyFilter{
		ProjectID: project.UID,
	}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	keys, pagination, err := service.LoadAPIKeysPaged(ctx, filter, pageable)

	require.NoError(t, err)
	require.Empty(t, keys)
	require.False(t, pagination.HasNextPage)
	require.False(t, pagination.HasPreviousPage)
}

func TestLoadAPIKeysPaged_SinglePage(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create only 3 API keys
	for i := 0; i < 3; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("Key %d", i),
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	// Fetch with per_page = 10 (more than available)
	filter := &datastore.ApiKeyFilter{
		ProjectID: project.UID,
	}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	keys, pagination, err := service.LoadAPIKeysPaged(ctx, filter, pageable)

	require.NoError(t, err)
	require.Len(t, keys, 3)
	require.False(t, pagination.HasNextPage)
	require.False(t, pagination.HasPreviousPage)
}

func TestLoadAPIKeysPaged_FilterByProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, org, project1 := seedTestData(t, db)

	// Create a second project
	projectRepo := postgres.NewProjectRepo(db)
	projectConfig := datastore.DefaultProjectConfig
	project2 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Project 2",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         &projectConfig,
	}
	err := projectRepo.CreateProject(ctx, project2)
	require.NoError(t, err)

	service := createAPIKeyService(t, db)

	// Create keys for project1
	for i := 0; i < 5; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("P1 Key %d", i),
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project1.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	// Create keys for project2
	for i := 0; i < 3; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("P2 Key %d", i),
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project2.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	// Fetch only project1 keys
	filter := &datastore.ApiKeyFilter{
		ProjectID: project1.UID,
	}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	keys, _, err := service.LoadAPIKeysPaged(ctx, filter, pageable)

	require.NoError(t, err)
	require.Len(t, keys, 5)

	// Verify all keys belong to project1
	for _, key := range keys {
		require.Equal(t, project1.UID, key.Role.Project)
	}
}

func TestLoadAPIKeysPaged_FilterByUser(t *testing.T) {
	db, ctx := setupTestDB(t)
	user1, _, project := seedTestData(t, db)

	// Create a second user
	userRepo := postgres.NewUserRepo(db)
	user2 := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "User",
		LastName:  "Two",
		Email:     "user2@example.com",
	}
	err := userRepo.CreateUser(ctx, user2)
	require.NoError(t, err)

	service := createAPIKeyService(t, db)

	// Create keys for user1
	for i := 0; i < 5; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("U1 Key %d", i),
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user1.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	// Create keys for user2
	for i := 0; i < 3; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("U2 Key %d", i),
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user2.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	// Fetch only user1 keys
	filter := &datastore.ApiKeyFilter{
		UserID: user1.UID,
	}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	keys, _, err := service.LoadAPIKeysPaged(ctx, filter, pageable)

	require.NoError(t, err)
	require.Len(t, keys, 5)

	// Verify all keys belong to user1
	for _, key := range keys {
		require.Equal(t, user1.UID, key.UserID)
	}
}

func TestLoadAPIKeysPaged_FilterByKeyType(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create personal keys
	for i := 0; i < 3; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("Personal Key %d", i),
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	// Create project keys
	for i := 0; i < 2; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("Project Key %d", i),
			Type:   datastore.ProjectKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	// Fetch only personal keys
	filter := &datastore.ApiKeyFilter{
		KeyType: datastore.PersonalKey,
	}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	keys, _, err := service.LoadAPIKeysPaged(ctx, filter, pageable)

	require.NoError(t, err)
	require.Len(t, keys, 3)

	// Verify all are personal keys
	for _, key := range keys {
		require.Equal(t, datastore.PersonalKey, key.Type)
	}
}

func TestLoadAPIKeysPaged_MultipleFilters(t *testing.T) {
	db, ctx := setupTestDB(t)
	user1, _, project := seedTestData(t, db)

	// Create a second user
	userRepo := postgres.NewUserRepo(db)
	user2 := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "User",
		LastName:  "Two",
		Email:     "user2@example.com",
	}
	err := userRepo.CreateUser(ctx, user2)
	require.NoError(t, err)

	service := createAPIKeyService(t, db)

	// Create keys with different combinations
	// user1, personal
	for i := 0; i < 3; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   "Key",
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user1.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	// user1, project
	apiKey := &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Key",
		Type:   datastore.ProjectKey,
		MaskID: ulid.Make().String(),
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   ulid.Make().String(),
		Salt:   ulid.Make().String(),
		UserID: user1.UID,
	}
	err = service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// user2, personal
	apiKey = &datastore.APIKey{
		UID:    ulid.Make().String(),
		Name:   "Key",
		Type:   datastore.PersonalKey,
		MaskID: ulid.Make().String(),
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
		Hash:   ulid.Make().String(),
		Salt:   ulid.Make().String(),
		UserID: user2.UID,
	}
	err = service.CreateAPIKey(ctx, apiKey)
	require.NoError(t, err)

	// Filter: user1 + personal keys
	filter := &datastore.ApiKeyFilter{
		UserID:  user1.UID,
		KeyType: datastore.PersonalKey,
	}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	keys, _, err := service.LoadAPIKeysPaged(ctx, filter, pageable)

	require.NoError(t, err)
	require.Len(t, keys, 3) // Only user1's personal keys

	// Verify all match both filters
	for _, key := range keys {
		require.Equal(t, user1.UID, key.UserID)
		require.Equal(t, datastore.PersonalKey, key.Type)
	}
}

func TestLoadAPIKeysPaged_NoOverlap(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create 20 API keys
	for i := 0; i < 20; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("Key %d", i),
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	filter := &datastore.ApiKeyFilter{
		ProjectID: project.UID,
	}

	// Fetch page 1
	pageable1 := datastore.Pageable{
		PerPage:   5,
		Direction: datastore.Next,
	}
	pageable1.SetCursors()
	page1, pagination1, err := service.LoadAPIKeysPaged(ctx, filter, pageable1)
	require.NoError(t, err)

	// Fetch page 2
	page2, pagination2, err := service.LoadAPIKeysPaged(ctx, filter, datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Next,
		NextCursor: pagination1.NextPageCursor,
	})
	require.NoError(t, err)

	// Fetch page 3
	page3, _, err := service.LoadAPIKeysPaged(ctx, filter, datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Next,
		NextCursor: pagination2.NextPageCursor,
	})
	require.NoError(t, err)

	// Verify no overlap
	allKeys := append(append(page1, page2...), page3...)
	seen := make(map[string]bool)
	for _, key := range allKeys {
		require.False(t, seen[key.UID], "Duplicate key found: %s", key.UID)
		seen[key.UID] = true
	}
}

func TestLoadAPIKeysPaged_ConsistentOrdering(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create 10 API keys
	for i := 0; i < 10; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("Key %d", i),
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	filter := &datastore.ApiKeyFilter{
		ProjectID: project.UID,
	}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	// Fetch twice
	keys1, _, err := service.LoadAPIKeysPaged(ctx, filter, pageable)
	require.NoError(t, err)

	keys2, _, err := service.LoadAPIKeysPaged(ctx, filter, pageable)
	require.NoError(t, err)

	// Verify same order
	require.Len(t, keys1, len(keys2))
	for i := range keys1 {
		require.Equal(t, keys1[i].UID, keys2[i].UID)
	}
}

func TestLoadAPIKeysPaged_BackwardPagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	user, _, project := seedTestData(t, db)
	service := createAPIKeyService(t, db)

	// Create 15 API keys
	for i := 0; i < 15; i++ {
		apiKey := &datastore.APIKey{
			UID:    ulid.Make().String(),
			Name:   fmt.Sprintf("Key %d", i),
			Type:   datastore.PersonalKey,
			MaskID: ulid.Make().String(),
			Role: auth.Role{
				Type:    auth.RoleProjectAdmin,
				Project: project.UID,
			},
			Hash:   ulid.Make().String(),
			Salt:   ulid.Make().String(),
			UserID: user.UID,
		}
		err := service.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
	}

	filter := &datastore.ApiKeyFilter{
		ProjectID: project.UID,
	}

	// Fetch first page forward
	pageable1 := datastore.Pageable{
		PerPage:   5,
		Direction: datastore.Next,
	}
	pageable1.SetCursors()
	page1Forward, pagination1, err := service.LoadAPIKeysPaged(ctx, filter, pageable1)
	require.NoError(t, err)
	require.Len(t, page1Forward, 5)

	// Fetch second page forward
	pageable2 := datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Next,
		NextCursor: pagination1.NextPageCursor,
	}
	page2Forward, pagination2, err := service.LoadAPIKeysPaged(ctx, filter, pageable2)
	require.NoError(t, err)
	require.Len(t, page2Forward, 5)

	// Fetch backward from page 2 (should get page 1 again)
	pageable3 := datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Prev,
		PrevCursor: pagination2.PrevPageCursor,
	}
	page1Backward, _, err := service.LoadAPIKeysPaged(ctx, filter, pageable3)
	require.NoError(t, err)
	require.Len(t, page1Backward, 5)

	// Verify we got the same keys as the first page
	for i := range page1Forward {
		require.Equal(t, page1Forward[i].UID, page1Backward[i].UID)
	}
}
