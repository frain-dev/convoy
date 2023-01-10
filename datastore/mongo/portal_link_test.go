//go:build integration
// +build integration

package mongo

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_CreatePortalLink(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	portalLinkRepo := NewPortalLinkRepo(store)
	portalLink := generatePortalLink(t)

	require.NoError(t, portalLinkRepo.CreatePortalLink(context.Background(), portalLink))

	newPortalLink, err := portalLinkRepo.FindPortalLinkByID(context.Background(), portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	require.Equal(t, portalLink.UID, newPortalLink.UID)
	require.Equal(t, portalLink.Name, newPortalLink.Name)
	require.Equal(t, portalLink.ProjectID, newPortalLink.ProjectID)
	require.Equal(t, portalLink.Endpoints[0], newPortalLink.Endpoints[0])
}

func Test_FindPortalLinkByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	portalLinkRepo := NewPortalLinkRepo(store)
	portalLink := generatePortalLink(t)

	_, err := portalLinkRepo.FindPortalLinkByID(context.Background(), portalLink.ProjectID, portalLink.UID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrPortalLinkNotFound))

	require.NoError(t, portalLinkRepo.CreatePortalLink(context.Background(), portalLink))

	newPortalLink, err := portalLinkRepo.FindPortalLinkByID(context.Background(), portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	require.Equal(t, portalLink.UID, newPortalLink.UID)
	require.Equal(t, portalLink.Name, newPortalLink.Name)
}

func Test_FindPortalLinkByToken(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	portalLinkRepo := NewPortalLinkRepo(store)
	portalLink := generatePortalLink(t)

	_, err := portalLinkRepo.FindPortalLinkByToken(context.Background(), portalLink.Token)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrPortalLinkNotFound))

	require.NoError(t, portalLinkRepo.CreatePortalLink(context.Background(), portalLink))

	newPortalLink, err := portalLinkRepo.FindPortalLinkByToken(context.Background(), portalLink.Token)
	require.NoError(t, err)

	require.Equal(t, portalLink.UID, newPortalLink.UID)
	require.Equal(t, portalLink.Token, newPortalLink.Token)
	require.Equal(t, portalLink.Name, newPortalLink.Name)
}

func Test_UpdatePortalLink(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	portalLinkRepo := NewPortalLinkRepo(store)
	portalLink := generatePortalLink(t)

	require.NoError(t, portalLinkRepo.CreatePortalLink(context.Background(), portalLink))

	endpoints := []string{uuid.NewString()}
	portalLink.Endpoints = endpoints

	require.NoError(t, portalLinkRepo.UpdatePortalLink(context.Background(), portalLink.ProjectID, portalLink))

	newPortalLink, err := portalLinkRepo.FindPortalLinkByID(context.Background(), portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	require.Equal(t, endpoints[0], newPortalLink.Endpoints[0])
}

func Test_RevokePortalLink(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	portalLinkRepo := NewPortalLinkRepo(store)
	portalLink := generatePortalLink(t)

	require.NoError(t, portalLinkRepo.CreatePortalLink(context.Background(), portalLink))

	_, err := portalLinkRepo.FindPortalLinkByID(context.Background(), portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	require.NoError(t, portalLinkRepo.RevokePortalLink(context.Background(), portalLink.ProjectID, portalLink.UID))

	_, err = portalLinkRepo.FindPortalLinkByID(context.Background(), portalLink.ProjectID, portalLink.UID)

	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrPortalLinkNotFound))
}

func Test_LoadPortalLinksPaged(t *testing.T) {
	type Expected struct {
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name     string
		filter   *datastore.FilterBy
		pageData datastore.Pageable
		count    int
		expected Expected
	}{
		{
			name:     "Load Portal Links Paged - 10 records",
			filter:   &datastore.FilterBy{},
			pageData: datastore.Pageable{Page: 1, PerPage: 3, Sort: -1},
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
			filter:   &datastore.FilterBy{},
			pageData: datastore.Pageable{Page: 2, PerPage: 4, Sort: -1},
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
			filter:   &datastore.FilterBy{},
			pageData: datastore.Pageable{Page: 1, PerPage: 3, Sort: -1},
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

			store := getStore(db)
			portalLinkRepo := NewPortalLinkRepo(store)
			projectID := uuid.NewString()

			for i := 0; i < tc.count; i++ {
				portalLink := &datastore.PortalLink{
					UID:       uuid.NewString(),
					ProjectID: projectID,
					Endpoints: []string{uuid.NewString()},
					Token:     uniuri.NewLen(5),
				}

				require.NoError(t, portalLinkRepo.CreatePortalLink(context.Background(), portalLink))
			}

			_, pageable, err := portalLinkRepo.LoadPortalLinksPaged(context.Background(), projectID, tc.filter, tc.pageData)

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

func generatePortalLink(t *testing.T) *datastore.PortalLink {
	return &datastore.PortalLink{
		UID:       uuid.NewString(),
		ProjectID: uuid.NewString(),
		Name:      fmt.Sprintf("Test-%s", uuid.NewString()),
		Token:     uniuri.NewLen(5),
		Endpoints: []string{uuid.NewString(), uuid.NewString()},
	}
}
