package sources

import (
	"fmt"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestLoadSourcesPaged_FirstPage(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create multiple sources
	for i := 0; i < 5; i++ {
		SeedSource(t, db, project, datastore.NoopVerifier)
	}

	filter := &datastore.SourceFilter{}
	pageable := datastore.Pageable{
		PerPage:    3,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, pagination, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 3)
	require.True(t, pagination.HasNextPage)
	require.NotEmpty(t, pagination.NextPageCursor)
}

func TestLoadSourcesPaged_ForwardPagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create sources
	for i := 0; i < 10; i++ {
		SeedSource(t, db, project, datastore.NoopVerifier)
	}

	// First page
	filter := &datastore.SourceFilter{}
	pageable := datastore.Pageable{
		PerPage:    3,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	page1, pagination1, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, page1, 3)
	require.True(t, pagination1.HasNextPage)

	// Second page
	pageable.NextCursor = pagination1.NextPageCursor
	page2, pagination2, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, page2, 3)
	require.True(t, pagination2.HasNextPage)

	// Verify pages are different
	require.NotEqual(t, page1[0].UID, page2[0].UID)
}

func TestLoadSourcesPaged_BackwardPagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create sources
	for i := 0; i < 10; i++ {
		SeedSource(t, db, project, datastore.NoopVerifier)
	}

	// Get second page first
	filter := &datastore.SourceFilter{}
	pageableNext := datastore.Pageable{
		PerPage:    3,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	_, pagination1, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageableNext)
	require.NoError(t, err)

	pageableNext.NextCursor = pagination1.NextPageCursor
	page2, pagination2, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageableNext)
	require.NoError(t, err)
	require.Len(t, page2, 3)

	// Now paginate backward
	pageablePrev := datastore.Pageable{
		PerPage:    3,
		Direction:  datastore.Prev,
		PrevCursor: pagination2.PrevPageCursor,
	}

	pagePrev, paginationPrev, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageablePrev)
	require.NoError(t, err)
	require.Len(t, pagePrev, 3)
	// Since we paginated back to the first page, there should be no previous page
	require.False(t, paginationPrev.HasPreviousPage)
	// But there should be a next page (page 2 and beyond)
	require.True(t, paginationPrev.HasNextPage)
}

func TestLoadSourcesPaged_FilterByType(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create HTTP sources
	for i := 0; i < 3; i++ {
		SeedSource(t, db, project, datastore.NoopVerifier)
	}

	// Create PubSub sources
	for i := 0; i < 2; i++ {
		source := &datastore.Source{
			UID:       ulid.Make().String(),
			Name:      fmt.Sprintf("PubSubSource-%d", i),
			Type:      datastore.PubSubSource,
			MaskID:    ulid.Make().String(),
			Provider:  datastore.GithubSourceProvider,
			ProjectID: project.UID,
			PubSub: &datastore.PubSubConfig{
				Type:    datastore.SqsPubSub,
				Workers: 3,
			},
			Verifier: &datastore.VerifierConfig{
				Type: datastore.NoopVerifier,
			},
		}
		err := service.CreateSource(ctx, source)
		require.NoError(t, err)
	}

	// Filter by HTTP type
	filter := &datastore.SourceFilter{
		Type: string(datastore.HTTPSource),
	}
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 3)
	for _, s := range sources {
		require.Equal(t, datastore.HTTPSource, s.Type)
	}

	// Filter by PubSub type
	filter.Type = string(datastore.PubSubSource)
	sources, _, err = service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 2)
	for _, s := range sources {
		require.Equal(t, datastore.PubSubSource, s.Type)
	}
}

func TestLoadSourcesPaged_FilterByProvider(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create GitHub sources
	for i := 0; i < 3; i++ {
		source := &datastore.Source{
			UID:       ulid.Make().String(),
			Name:      fmt.Sprintf("GithubSource-%d", i),
			Type:      datastore.HTTPSource,
			MaskID:    ulid.Make().String(),
			Provider:  datastore.GithubSourceProvider,
			ProjectID: project.UID,
			Verifier: &datastore.VerifierConfig{
				Type: datastore.NoopVerifier,
			},
		}
		err := service.CreateSource(ctx, source)
		require.NoError(t, err)
	}

	// Create Shopify sources
	for i := 0; i < 2; i++ {
		source := &datastore.Source{
			UID:       ulid.Make().String(),
			Name:      fmt.Sprintf("ShopifySource-%d", i),
			Type:      datastore.HTTPSource,
			MaskID:    ulid.Make().String(),
			Provider:  datastore.ShopifySourceProvider,
			ProjectID: project.UID,
			Verifier: &datastore.VerifierConfig{
				Type: datastore.NoopVerifier,
			},
		}
		err := service.CreateSource(ctx, source)
		require.NoError(t, err)
	}

	// Filter by GitHub provider
	filter := &datastore.SourceFilter{
		Provider: string(datastore.GithubSourceProvider),
	}
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 3)
	for _, s := range sources {
		require.Equal(t, datastore.GithubSourceProvider, s.Provider)
	}

	// Filter by Shopify provider
	filter.Provider = string(datastore.ShopifySourceProvider)
	sources, _, err = service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 2)
	for _, s := range sources {
		require.Equal(t, datastore.ShopifySourceProvider, s.Provider)
	}
}

func TestLoadSourcesPaged_SearchByQuery(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create sources with specific names
	names := []string{"ProductionAPI", "StagingAPI", "DevelopmentAPI", "TestWebhook", "DemoSource"}
	for _, name := range names {
		source := &datastore.Source{
			UID:       ulid.Make().String(),
			Name:      name,
			Type:      datastore.HTTPSource,
			MaskID:    ulid.Make().String(),
			Provider:  datastore.GithubSourceProvider,
			ProjectID: project.UID,
			Verifier: &datastore.VerifierConfig{
				Type: datastore.NoopVerifier,
			},
		}
		err := service.CreateSource(ctx, source)
		require.NoError(t, err)
	}

	// Search for "API"
	filter := &datastore.SourceFilter{
		Query: "API",
	}
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 3)
	for _, s := range sources {
		require.Contains(t, s.Name, "API")
	}

	// Search for "test" (case insensitive)
	filter.Query = "test"
	sources, _, err = service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.Contains(t, sources[0].Name, "Test")
}

func TestLoadSourcesPaged_CombinedFilters(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create various sources
	sources := []struct {
		name     string
		typ      datastore.SourceType
		provider datastore.SourceProvider
	}{
		{"GithubProd", datastore.HTTPSource, datastore.GithubSourceProvider},
		{"GithubDev", datastore.HTTPSource, datastore.GithubSourceProvider},
		{"ShopifyProd", datastore.HTTPSource, datastore.ShopifySourceProvider},
		{"PubSubGithub", datastore.PubSubSource, datastore.GithubSourceProvider},
	}

	for _, s := range sources {
		source := &datastore.Source{
			UID:       ulid.Make().String(),
			Name:      s.name,
			Type:      s.typ,
			MaskID:    ulid.Make().String(),
			Provider:  s.provider,
			ProjectID: project.UID,
			Verifier: &datastore.VerifierConfig{
				Type: datastore.NoopVerifier,
			},
		}
		if s.typ == datastore.PubSubSource {
			source.PubSub = &datastore.PubSubConfig{
				Type:    datastore.SqsPubSub,
				Workers: 3,
			}
		}
		err := service.CreateSource(ctx, source)
		require.NoError(t, err)
	}

	// Filter: HTTP + GitHub + "Prod"
	filter := &datastore.SourceFilter{
		Type:     string(datastore.HTTPSource),
		Provider: string(datastore.GithubSourceProvider),
		Query:    "Prod",
	}
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	result, _, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "GithubProd", result[0].Name)
}

func TestLoadSourcesPaged_EmptyResult(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Don't create any sources

	filter := &datastore.SourceFilter{}
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, pagination, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Empty(t, sources)
	require.False(t, pagination.HasNextPage)
	require.False(t, pagination.HasPreviousPage)
}

func TestLoadSourcesPaged_SinglePage(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create fewer sources than page size
	for i := 0; i < 3; i++ {
		SeedSource(t, db, project, datastore.NoopVerifier)
	}

	filter := &datastore.SourceFilter{}
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, pagination, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 3)
	require.False(t, pagination.HasNextPage)
}

func TestLoadSourcesPaged_IncludesVerifiers(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create sources with different verifiers
	SeedSource(t, db, project, datastore.APIKeyVerifier)
	SeedSource(t, db, project, datastore.BasicAuthVerifier)
	SeedSource(t, db, project, datastore.HMacVerifier)

	filter := &datastore.SourceFilter{}
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 3)

	// Verify all verifiers are included
	for _, s := range sources {
		require.NotNil(t, s.Verifier)
		require.NotEmpty(t, s.Verifier.Type)
	}
}

func TestLoadSourcesPaged_ProjectIsolation(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project1 := seedTestData(t, db)
	project2 := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create sources in project1
	for i := 0; i < 3; i++ {
		SeedSource(t, db, project1, datastore.NoopVerifier)
	}

	// Create sources in project2
	for i := 0; i < 2; i++ {
		SeedSource(t, db, project2, datastore.NoopVerifier)
	}

	filter := &datastore.SourceFilter{}
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	// Query project1
	sources1, _, err := service.LoadSourcesPaged(ctx, project1.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources1, 3)
	for _, s := range sources1 {
		require.Equal(t, project1.UID, s.ProjectID)
	}

	// Query project2
	sources2, _, err := service.LoadSourcesPaged(ctx, project2.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources2, 2)
	for _, s := range sources2 {
		require.Equal(t, project2.UID, s.ProjectID)
	}
}

func TestLoadSourcesPaged_ExcludesDeletedSources(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create sources
	source1 := SeedSource(t, db, project, datastore.NoopVerifier)
	source2 := SeedSource(t, db, project, datastore.NoopVerifier)
	SeedSource(t, db, project, datastore.NoopVerifier)

	// Delete one source
	err := service.DeleteSourceByID(ctx, project.UID, source1.UID, source1.VerifierID)
	require.NoError(t, err)

	// Delete another source
	err = service.DeleteSourceByID(ctx, project.UID, source2.UID, source2.VerifierID)
	require.NoError(t, err)

	// Query should only return non-deleted source
	filter := &datastore.SourceFilter{}
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadSourcesPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.NotEqual(t, source1.UID, sources[0].UID)
	require.NotEqual(t, source2.UID, sources[0].UID)
}
