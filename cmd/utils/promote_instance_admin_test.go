package utils

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func Test_resolveOrgIDForPromotion_explicitOrgID(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOrganisationMemberRepository(ctrl)

	id, err := resolveOrgIDForPromotion(ctx, repo, "user-1", "  org-trimmed  ")
	require.NoError(t, err)
	require.Equal(t, "org-trimmed", id)
}

func Test_resolveOrgIDForPromotion_noMemberships(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOrganisationMemberRepository(ctrl)

	repo.EXPECT().
		LoadUserOrganisationsPaged(gomock.Any(), "user-1", gomock.Any()).
		Return(nil, datastore.PaginationData{HasNextPage: false}, nil)

	_, err := resolveOrgIDForPromotion(ctx, repo, "user-1", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no organisation memberships")
}

func Test_resolveOrgIDForPromotion_singleOrg(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOrganisationMemberRepository(ctrl)

	repo.EXPECT().
		LoadUserOrganisationsPaged(gomock.Any(), "user-1", gomock.Any()).
		Return(
			[]datastore.Organisation{{UID: "org-a", Name: "A"}},
			datastore.PaginationData{HasNextPage: false},
			nil,
		)

	id, err := resolveOrgIDForPromotion(ctx, repo, "user-1", "")
	require.NoError(t, err)
	require.Equal(t, "org-a", id)
}

func Test_resolveOrgIDForPromotion_multipleOrgs_errors(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOrganisationMemberRepository(ctrl)

	repo.EXPECT().
		LoadUserOrganisationsPaged(gomock.Any(), "user-1", gomock.Any()).
		Return(
			[]datastore.Organisation{
				{UID: "org-a", Name: "Alpha"},
				{UID: "org-b", Name: "Beta"},
			},
			datastore.PaginationData{HasNextPage: false},
			nil,
		)

	_, err := resolveOrgIDForPromotion(ctx, repo, "user-1", "")
	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "multiple organisations")
	require.Contains(t, msg, "set --org-id")
	require.Contains(t, msg, "org-a")
	require.Contains(t, msg, "Alpha")
	require.Contains(t, msg, "org-b")
	require.Contains(t, msg, "Beta")
}

func Test_loadAllUserOrganisations_singlePage(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOrganisationMemberRepository(ctrl)

	repo.EXPECT().
		LoadUserOrganisationsPaged(gomock.Any(), "user-1", gomock.Any()).
		Return(
			[]datastore.Organisation{{UID: "o1", Name: "One"}},
			datastore.PaginationData{HasNextPage: false},
			nil,
		)

	orgs, err := loadAllUserOrganisations(ctx, repo, "user-1")
	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, "o1", orgs[0].UID)
}

func Test_loadAllUserOrganisations_twoPages(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOrganisationMemberRepository(ctrl)

	gomock.InOrder(
		repo.EXPECT().
			LoadUserOrganisationsPaged(gomock.Any(), "user-1", gomock.Any()).
			Return(
				[]datastore.Organisation{{UID: "p1", Name: "First"}},
				datastore.PaginationData{HasNextPage: true, NextPageCursor: "cursor-2"},
				nil,
			),
		repo.EXPECT().
			LoadUserOrganisationsPaged(gomock.Any(), "user-1", gomock.Any()).
			Return(
				[]datastore.Organisation{{UID: "p2", Name: "Second"}},
				datastore.PaginationData{HasNextPage: false},
				nil,
			),
	)

	orgs, err := loadAllUserOrganisations(ctx, repo, "user-1")
	require.NoError(t, err)
	require.Len(t, orgs, 2)
	require.Equal(t, "p1", orgs[0].UID)
	require.Equal(t, "p2", orgs[1].UID)
}

func Test_loadAllUserOrganisations_repoError(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOrganisationMemberRepository(ctrl)

	repo.EXPECT().
		LoadUserOrganisationsPaged(gomock.Any(), "user-1", gomock.Any()).
		Return(nil, datastore.PaginationData{}, context.Canceled)

	_, err := loadAllUserOrganisations(ctx, repo, "user-1")
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "list user organisations"))
}

func Test_loadAllUserOrganisations_maxPagesExceeded(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOrganisationMemberRepository(ctrl)

	for i := 0; i < 50; i++ {
		repo.EXPECT().
			LoadUserOrganisationsPaged(gomock.Any(), "user-1", gomock.Any()).
			Return(
				[]datastore.Organisation{{UID: "org-x", Name: "X"}},
				datastore.PaginationData{HasNextPage: true, NextPageCursor: "more"},
				nil,
			)
	}

	_, err := loadAllUserOrganisations(ctx, repo, "user-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "too many organisation memberships")
}
