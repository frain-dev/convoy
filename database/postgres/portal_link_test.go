//go:build integration
// +build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func Test_CreatePortalLink(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	portalLinkRepo := NewPortalLinkRepo(db)
	portalLink := generatePortalLink(t, db)

	require.NoError(t, portalLinkRepo.CreatePortalLink(context.Background(), portalLink))

	newPortalLink, err := portalLinkRepo.FindPortalLinkByID(context.Background(), portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	newPortalLink.CreatedAt = time.Time{}
	newPortalLink.UpdatedAt = time.Time{}

	require.Equal(t, portalLink, newPortalLink)
}

func Test_FindPortalLinkByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	portalLinkRepo := NewPortalLinkRepo(db)
	portalLink := generatePortalLink(t, db)
	ctx := context.Background()

	_, err := portalLinkRepo.FindPortalLinkByID(ctx, portalLink.ProjectID, portalLink.UID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrPortalLinkNotFound))

	require.NoError(t, portalLinkRepo.CreatePortalLink(ctx, portalLink))

	newPortalLink, err := portalLinkRepo.FindPortalLinkByID(ctx, portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	newPortalLink.CreatedAt = time.Time{}
	newPortalLink.UpdatedAt = time.Time{}

	require.Equal(t, portalLink, newPortalLink)
}

func Test_FindPortalLinkByToken(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	portalLinkRepo := NewPortalLinkRepo(db)
	portalLink := generatePortalLink(t, db)
	ctx := context.Background()

	_, err := portalLinkRepo.FindPortalLinkByToken(ctx, portalLink.Token)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrPortalLinkNotFound))

	require.NoError(t, portalLinkRepo.CreatePortalLink(ctx, portalLink))

	newPortalLink, err := portalLinkRepo.FindPortalLinkByToken(ctx, portalLink.Token)
	require.NoError(t, err)

	newPortalLink.CreatedAt = time.Time{}
	newPortalLink.UpdatedAt = time.Time{}

	require.Equal(t, portalLink, newPortalLink)
}

func Test_UpdatePortalLink(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	portalLinkRepo := NewPortalLinkRepo(db)
	projectRepo := NewProjectRepo(db)
	endpointRepo := NewEndpointRepo(db)

	portalLink := generatePortalLink(t, db)
	ctx := context.Background()

	project, err := projectRepo.FetchProjectByID(ctx, portalLink.ProjectID)
	require.NoError(t, err)

	require.NoError(t, portalLinkRepo.CreatePortalLink(ctx, portalLink))

	portalLink.Name = "Updated-Test-Portal-Token"
	endpoint := generateEndpoint(project)

	err = endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	portalLink.Endpoints = []string{endpoint.UID}
	require.NoError(t, portalLinkRepo.UpdatePortalLink(ctx, portalLink.ProjectID, portalLink))

	newPortalLink, err := portalLinkRepo.FindPortalLinkByID(ctx, portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	total, _, err := portalLinkRepo.LoadPortalLinksPaged(ctx, project.UID, &datastore.FilterBy{EndpointIDs: []string{endpoint.UID}}, datastore.Pageable{Page: 1, PerPage: 10})

	require.Equal(t, 1, len(total))
	require.Equal(t, endpoint.UID, total[0].EndpointsMetadata[0].UID)

	newPortalLink.CreatedAt = time.Time{}
	newPortalLink.UpdatedAt = time.Time{}

	require.Equal(t, portalLink, newPortalLink)
}

func Test_RevokePortalLink(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	portalLinkRepo := NewPortalLinkRepo(db)
	portalLink := generatePortalLink(t, db)
	ctx := context.Background()

	require.NoError(t, portalLinkRepo.CreatePortalLink(ctx, portalLink))

	_, err := portalLinkRepo.FindPortalLinkByID(ctx, portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	require.NoError(t, portalLinkRepo.RevokePortalLink(ctx, portalLink.ProjectID, portalLink.UID))

	_, err = portalLinkRepo.FindPortalLinkByID(ctx, portalLink.ProjectID, portalLink.UID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrPortalLinkNotFound))
}

func Test_LoadPortalLinksPaged(t *testing.T) {
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
			name:     "Load Portal Links Paged - 10 records",
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
			name:     "Load Portal Links Paged - 12 records",
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
			name:     "Load Portal Links Paged - 5 records",
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
			endpoint := generateEndpoint(project)
			portalLinkRepo := NewPortalLinkRepo(db)
			NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint, project.UID)

			for i := 0; i < tc.count; i++ {
				portalLink := &datastore.PortalLink{
					UID:       ulid.Make().String(),
					ProjectID: project.UID,
					Name:      "Test-Portal-Link",
					Token:     ulid.Make().String(),
					Endpoints: []string{endpoint.UID},
				}
				require.NoError(t, portalLinkRepo.CreatePortalLink(context.Background(), portalLink))
			}

			_, pageable, err := portalLinkRepo.LoadPortalLinksPaged(context.Background(), project.UID, &datastore.FilterBy{EndpointID: endpoint.UID}, tc.pageData)

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

func generatePortalLink(t *testing.T, db database.Database) *datastore.PortalLink {
	project := seedProject(t, db)

	endpoint := generateEndpoint(project)
	err := NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	return &datastore.PortalLink{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		Name:      "Test-Portal-Link",
		Token:     ulid.Make().String(),
		Endpoints: []string{endpoint.UID},
	}
}
