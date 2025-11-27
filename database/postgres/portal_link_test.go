//go:build integration
// +build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
)

func Test_CreatePortalLink(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	repo := NewPortalLinkRepo(db)
	portalLink := generatePortalLink(t, db)

	require.NoError(t, repo.CreatePortalLink(context.Background(), portalLink))

	newPortalLink, err := repo.FindPortalLinkByID(context.Background(), portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	newPortalLink.CreatedAt = time.Time{}
	newPortalLink.UpdatedAt = time.Time{}

	require.Equal(t, portalLink.Name, newPortalLink.Name)
	require.Equal(t, portalLink.Token, newPortalLink.Token)
	require.Equal(t, portalLink.ProjectID, newPortalLink.ProjectID)
}

func Test_FindPortalLinkByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	repo := NewPortalLinkRepo(db)
	portalLink := generatePortalLink(t, db)
	ctx := context.Background()

	_, err := repo.FindPortalLinkByID(ctx, portalLink.ProjectID, portalLink.UID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrPortalLinkNotFound))

	require.NoError(t, repo.CreatePortalLink(ctx, portalLink))

	newPortalLink, err := repo.FindPortalLinkByID(ctx, portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	newPortalLink.CreatedAt = time.Time{}
	newPortalLink.UpdatedAt = time.Time{}

	require.Equal(t, portalLink.Name, newPortalLink.Name)
	require.Equal(t, portalLink.Token, newPortalLink.Token)
	require.Equal(t, portalLink.ProjectID, newPortalLink.ProjectID)
}

func Test_FindPortalLinkByToken(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	repo := NewPortalLinkRepo(db)
	portalLink := generatePortalLink(t, db)
	ctx := context.Background()

	_, err := repo.FindPortalLinkByToken(ctx, portalLink.Token)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrPortalLinkNotFound))

	require.NoError(t, repo.CreatePortalLink(ctx, portalLink))

	newPortalLink, err := repo.FindPortalLinkByToken(ctx, portalLink.Token)
	require.NoError(t, err)

	newPortalLink.CreatedAt = time.Time{}
	newPortalLink.UpdatedAt = time.Time{}

	require.Equal(t, portalLink.Name, newPortalLink.Name)
	require.Equal(t, portalLink.Token, newPortalLink.Token)
	require.Equal(t, portalLink.ProjectID, newPortalLink.ProjectID)
}

func Test_UpdatePortalLink(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	linkRepo := NewPortalLinkRepo(db)
	newProjectRepo := NewProjectRepo(db)
	newEndpointRepo := NewEndpointRepo(db)

	portalLink := generatePortalLink(t, db)
	ctx := context.Background()

	project, err := newProjectRepo.FetchProjectByID(ctx, portalLink.ProjectID)
	require.NoError(t, err)

	require.NoError(t, linkRepo.CreatePortalLink(ctx, portalLink))

	portalLink.Name = "Updated-Test-Portal-Token"
	endpoint := generateEndpoint(project)

	err = newEndpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	portalLink.Endpoints = []string{endpoint.UID}
	require.NoError(t, linkRepo.UpdatePortalLink(ctx, portalLink.ProjectID, portalLink))

	newPortalLink, err := linkRepo.FindPortalLinkByID(ctx, portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	total, _, err := linkRepo.LoadPortalLinksPaged(ctx, project.UID, &datastore.FilterBy{EndpointIDs: []string{endpoint.UID}}, datastore.Pageable{PerPage: 10, Direction: datastore.Next, NextCursor: fmt.Sprintf("%d", math.MaxInt)})
	require.NoError(t, err)

	require.Equal(t, 1, len(total))
	require.Equal(t, endpoint.UID, total[0].EndpointsMetadata[0].UID)

	newPortalLink.CreatedAt = time.Time{}
	newPortalLink.UpdatedAt = time.Time{}

	require.Equal(t, portalLink.Name, newPortalLink.Name)
	require.Equal(t, portalLink.Token, newPortalLink.Token)
	require.Equal(t, portalLink.ProjectID, newPortalLink.ProjectID)
}

func Test_RevokePortalLink(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	newPortalLinkRepo := NewPortalLinkRepo(db)
	portalLink := generatePortalLink(t, db)
	ctx := context.Background()

	require.NoError(t, newPortalLinkRepo.CreatePortalLink(ctx, portalLink))

	_, err := newPortalLinkRepo.FindPortalLinkByID(ctx, portalLink.ProjectID, portalLink.UID)
	require.NoError(t, err)

	require.NoError(t, newPortalLinkRepo.RevokePortalLink(ctx, portalLink.ProjectID, portalLink.UID))

	_, err = newPortalLinkRepo.FindPortalLinkByID(ctx, portalLink.ProjectID, portalLink.UID)
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
			pageData: datastore.Pageable{PerPage: 3},
			count:    10,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},

		{
			name:     "Load Portal Links Paged - 12 records",
			pageData: datastore.Pageable{PerPage: 4},
			count:    12,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 4,
				},
			},
		},

		{
			name:     "Load Portal Links Paged - 5 records",
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
			endpoint := generateEndpoint(project)
			repo := NewPortalLinkRepo(db)
			err := NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint, project.UID)
			require.NoError(t, err)

			for i := 0; i < tc.count; i++ {
				portalLink := &datastore.PortalLink{
					UID:       ulid.Make().String(),
					ProjectID: project.UID,
					AuthType:  datastore.PortalAuthTypeStaticToken,
					Name:      "Test-Portal-Link",
					Token:     ulid.Make().String(),
					Endpoints: []string{endpoint.UID},
				}
				require.NoError(t, repo.CreatePortalLink(context.Background(), portalLink))
			}

			_, pageable, err := repo.LoadPortalLinksPaged(context.Background(), project.UID, &datastore.FilterBy{EndpointID: endpoint.UID}, tc.pageData)

			require.NoError(t, err)
			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
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
		AuthType:  datastore.PortalAuthTypeStaticToken,
		Name:      "Test-Portal-Link",
		Token:     ulid.Make().String(),
		Endpoints: []string{endpoint.UID},
	}
}
