package subscriptions

import (
	"fmt"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestLoadSubscriptionsPaged_ForwardPagination(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	// Create 15 subscriptions for pagination testing
	createdSubs := make([]*datastore.Subscription, 15)
	for i := 0; i < 15; i++ {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.Name = fmt.Sprintf("Subscription %d", i+1)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)
		createdSubs[i] = sub
	}

	t.Run("should_fetch_first_page", func(t *testing.T) {
		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   5,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, paginationData, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.Len(t, subscriptions, 5)
		require.NotNil(t, paginationData)
		require.True(t, paginationData.HasNextPage)
		require.False(t, paginationData.HasPreviousPage)
	})

	t.Run("should_fetch_second_page_with_cursor", func(t *testing.T) {
		// First, get the first page to obtain the cursor
		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   5,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, paginationData, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.Len(t, subscriptions, 5)

		// Now fetch the second page using the next cursor
		pageable.NextCursor = paginationData.NextPageCursor

		subscriptions2, paginationData2, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.Len(t, subscriptions2, 5)
		require.True(t, paginationData2.HasNextPage)
		require.True(t, paginationData2.HasPreviousPage)

		// Verify no overlap
		firstPageIDs := make(map[string]bool)
		for _, sub := range subscriptions {
			firstPageIDs[sub.UID] = true
		}
		for _, sub := range subscriptions2 {
			require.False(t, firstPageIDs[sub.UID], "Found overlapping subscription between pages")
		}
	})

	t.Run("should_detect_last_page", func(t *testing.T) {
		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		// First page
		_, paginationData, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)

		// Go to next page
		pageable.NextCursor = paginationData.NextPageCursor
		subscriptions, paginationData2, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.LessOrEqual(t, len(subscriptions), 10)
		require.False(t, paginationData2.HasNextPage)
	})

	t.Run("should_handle_page_size_larger_than_results", func(t *testing.T) {
		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   100,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, paginationData, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(subscriptions), 15)
		require.False(t, paginationData.HasNextPage)
		require.False(t, paginationData.HasPreviousPage)
	})
}

func TestLoadSubscriptionsPaged_BackwardPagination(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	// Create 15 subscriptions
	for i := 0; i < 15; i++ {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.Name = fmt.Sprintf("Subscription %d", i+1)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)
	}

	t.Run("should_paginate_backward_from_cursor", func(t *testing.T) {
		// Get page 1 first
		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   5,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		page1Subs, page1Data, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.Len(t, page1Subs, 5)

		// Get page 2
		pageable.NextCursor = page1Data.NextPageCursor
		page2Subs, page2Data, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.Len(t, page2Subs, 5)

		// Now go backward using prev cursor
		pageable.PrevCursor = page2Data.PrevPageCursor
		pageable.Direction = datastore.Prev
		pageable.SetCursors()

		backPage1Subs, backPage1Data, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.Len(t, backPage1Subs, 5)
		require.True(t, backPage1Data.HasNextPage)
		require.False(t, backPage1Data.HasPreviousPage)

		// Verify no overlap
		page2IDs := make(map[string]bool)
		for _, sub := range page2Subs {
			page2IDs[sub.UID] = true
		}
		for _, sub := range backPage1Subs {
			require.False(t, page2IDs[sub.UID], "Found overlapping subscription between pages")
		}
	})

	t.Run("should_maintain_order_in_backward_pagination", func(t *testing.T) {
		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   5,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		// Get page 1
		_, page1Data, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)

		// Get page 2
		pageable.NextCursor = page1Data.NextPageCursor
		subscriptions, paginationData, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)

		// Verify results are sorted
		for i := 0; i < len(subscriptions)-1; i++ {
			require.True(t, subscriptions[i].UID >= subscriptions[i+1].UID, "Results should be sorted by UID descending")
		}

		require.NotEmpty(t, paginationData.PrevPageCursor)
	})
}

func TestLoadSubscriptionsPaged_Filtering(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	// Create endpoint-specific subscriptions
	endpoint2 := seedEndpoint(t, db, project)
	endpoint3 := seedEndpoint(t, db, project)

	for i := 0; i < 3; i++ {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.Name = fmt.Sprintf("Endpoint1 Subscription %d", i+1)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)
	}

	for i := 0; i < 2; i++ {
		sub := createTestSubscription(project, endpoint2, source)
		sub.UID = ulid.Make().String()
		sub.Name = fmt.Sprintf("Endpoint2 Subscription %d", i+1)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)
	}

	for i := 0; i < 2; i++ {
		sub := createTestSubscription(project, endpoint3, source)
		sub.UID = ulid.Make().String()
		sub.Name = fmt.Sprintf("Endpoint3 Subscription %d", i+1)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)
	}

	t.Run("should_filter_by_single_endpoint", func(t *testing.T) {
		filter := &datastore.FilterBy{
			EndpointIDs: []string{endpoint.UID},
		}
		pageable := datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, _, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(subscriptions), 3)

		// Verify all subscriptions belong to the filtered endpoint
		for _, sub := range subscriptions {
			require.Equal(t, endpoint.UID, sub.EndpointID)
		}
	})

	t.Run("should_filter_by_multiple_endpoints", func(t *testing.T) {
		filter := &datastore.FilterBy{
			EndpointIDs: []string{endpoint2.UID, endpoint3.UID},
		}
		pageable := datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, _, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(subscriptions), 4)

		// Verify all subscriptions belong to one of the filtered endpoints
		allowedEndpoints := map[string]bool{
			endpoint2.UID: true,
			endpoint3.UID: true,
		}
		for _, sub := range subscriptions {
			require.True(t, allowedEndpoints[sub.EndpointID], "Subscription endpoint not in filter list")
		}
	})

	t.Run("should_filter_by_subscription_name", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		sub.UID = ulid.Make().String()
		sub.Name = "UniqueSubscriptionName"
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		filter := &datastore.FilterBy{
			SubscriptionName: "UniqueSubscriptionName",
		}
		pageable := datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, _, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.NotEmpty(t, subscriptions)

		found := false
		for _, s := range subscriptions {
			if s.Name == "UniqueSubscriptionName" {
				found = true
				break
			}
		}
		require.True(t, found, "Should find subscription with unique name")
	})

	t.Run("should_combine_endpoint_and_name_filters", func(t *testing.T) {
		filter := &datastore.FilterBy{
			EndpointIDs:      []string{endpoint.UID},
			SubscriptionName: "Endpoint1",
		}
		pageable := datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, _, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.NotEmpty(t, subscriptions)

		// Verify all match both filters
		for _, sub := range subscriptions {
			require.Equal(t, endpoint.UID, sub.EndpointID)
			require.Contains(t, sub.Name, "Endpoint1")
		}
	})
}

func TestLoadSubscriptionsPaged_EdgeCases(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_return_empty_results_for_no_subscriptions", func(t *testing.T) {
		// Create a new project with no subscriptions
		user := seedUser(t, db)
		org := seedOrganisation(t, db, user)
		newProject := seedProject(t, db, org)

		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, paginationData, err := service.LoadSubscriptionsPaged(ctx, newProject.UID, filter, pageable)
		require.NoError(t, err)
		require.Empty(t, subscriptions)
		require.False(t, paginationData.HasNextPage)
		require.False(t, paginationData.HasPreviousPage)
	})

	t.Run("should_handle_single_result", func(t *testing.T) {
		// Create a new project with exactly one subscription
		user := seedUser(t, db)
		org := seedOrganisation(t, db, user)
		newProject := seedProject(t, db, org)
		newEndpoint := seedEndpoint(t, db, newProject)
		newSource := seedSource(t, db, newProject)

		sub := createTestSubscription(newProject, newEndpoint, newSource)
		err := service.CreateSubscription(ctx, newProject.UID, sub)
		require.NoError(t, err)

		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, paginationData, err := service.LoadSubscriptionsPaged(ctx, newProject.UID, filter, pageable)
		require.NoError(t, err)
		require.Len(t, subscriptions, 1)
		require.False(t, paginationData.HasNextPage)
		require.False(t, paginationData.HasPreviousPage)
	})

	t.Run("should_handle_results_exactly_equal_to_page_size", func(t *testing.T) {
		// Create exactly 5 subscriptions
		user := seedUser(t, db)
		org := seedOrganisation(t, db, user)
		newProject := seedProject(t, db, org)
		newEndpoint := seedEndpoint(t, db, newProject)
		newSource := seedSource(t, db, newProject)

		for i := 0; i < 5; i++ {
			sub := createTestSubscription(newProject, newEndpoint, newSource)
			sub.UID = ulid.Make().String()
			err := service.CreateSubscription(ctx, newProject.UID, sub)
			require.NoError(t, err)
		}

		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   5,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, paginationData, err := service.LoadSubscriptionsPaged(ctx, newProject.UID, filter, pageable)
		require.NoError(t, err)
		require.Len(t, subscriptions, 5)
		require.False(t, paginationData.HasNextPage)
	})

	t.Run("should_not_include_deleted_subscriptions", func(t *testing.T) {
		sub1 := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub1)
		require.NoError(t, err)

		sub2 := createTestSubscription(project, endpoint, source)
		sub2.UID = ulid.Make().String()
		err = service.CreateSubscription(ctx, project.UID, sub2)
		require.NoError(t, err)

		// Delete one subscription
		err = service.DeleteSubscription(ctx, project.UID, sub1)
		require.NoError(t, err)

		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   100,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, _, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)

		// Verify deleted subscription is not in results
		for _, sub := range subscriptions {
			require.NotEqual(t, sub1.UID, sub.UID, "Deleted subscription should not be in results")
		}
	})
}

func TestLoadSubscriptionsPaged_MetadataVerification(t *testing.T) {
	db, ctx, service := setupTestDB(t)
	defer db.GetConn().Close()

	project, endpoint, source, _ := seedTestData(t, db)

	t.Run("should_populate_pagination_metadata_accurately", func(t *testing.T) {
		// Create 12 subscriptions
		for i := 0; i < 12; i++ {
			sub := createTestSubscription(project, endpoint, source)
			sub.UID = ulid.Make().String()
			err := service.CreateSubscription(ctx, project.UID, sub)
			require.NoError(t, err)
		}

		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   5,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, paginationData, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.Len(t, subscriptions, 5)
		require.NotNil(t, paginationData)
		require.Equal(t, int64(5), paginationData.PerPage)
		require.NotEmpty(t, paginationData.NextPageCursor)
	})

	t.Run("should_populate_endpoint_and_source_metadata", func(t *testing.T) {
		sub := createTestSubscription(project, endpoint, source)
		err := service.CreateSubscription(ctx, project.UID, sub)
		require.NoError(t, err)

		filter := &datastore.FilterBy{}
		pageable := datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		}
		pageable.SetCursors()

		subscriptions, _, err := service.LoadSubscriptionsPaged(ctx, project.UID, filter, pageable)
		require.NoError(t, err)
		require.NotEmpty(t, subscriptions)

		// Verify metadata is populated
		for _, sub := range subscriptions {
			if sub.EndpointID != "" {
				require.NotNil(t, sub.Endpoint)
				require.Equal(t, sub.EndpointID, sub.Endpoint.UID)
			}
			if sub.SourceID != "" {
				require.NotNil(t, sub.Source)
				require.Equal(t, sub.SourceID, sub.Source.UID)
			}
		}
	})
}
