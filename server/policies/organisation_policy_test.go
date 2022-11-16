package policies

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func Test_OrganisationPolicy_Get(t *testing.T) {
	tests := map[string]struct {
		authCtx       *auth.AuthenticatedUser
		organisation  *datastore.Organisation
		storeFn       func(*OrganisationPolicy)
		wantErr       bool
		expectedError error
	}{
		"should_fail_when_user_is_not_a_member_of_the_organisation": {
			authCtx: &auth.AuthenticatedUser{
				User: &datastore.User{
					UID: "randomstring",
				},
			},
			organisation: &datastore.Organisation{
				UID: "randomstring",
			},
			storeFn: func(orgP *OrganisationPolicy) {
				orgMem := orgP.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)

				orgMem.EXPECT().
					FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("Failed"))
			},
			wantErr:       true,
			expectedError: ErrNotAllowed,
		},
		"should_fail_when_user_is_not_a_super_user": {
			authCtx: &auth.AuthenticatedUser{
				User: &datastore.User{
					UID: "randomstring",
				},
			},
			organisation: &datastore.Organisation{
				UID: "randomstring",
			},
			storeFn: func(orgP *OrganisationPolicy) {
				orgMem := orgP.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)

				orgMem.EXPECT().
					FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.OrganisationMember{
						Role: auth.Role{Type: auth.RoleAPI},
					}, nil)
			},
			wantErr:       true,
			expectedError: ErrNotAllowed,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Arrange.
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			policy := &OrganisationPolicy{
				orgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
			}
			authCtx := context.WithValue(context.Background(), AuthCtxKey, tc.authCtx)

			if tc.storeFn != nil {
				tc.storeFn(policy)
			}

			// Act.
			err := policy.Get(authCtx, tc.organisation)

			// Assert.
			if tc.wantErr {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_OrganisationPolicy_Update(t *testing.T) {
	tests := map[string]struct {
		authCtx       *auth.AuthenticatedUser
		organisation  *datastore.Organisation
		storeFn       func(*OrganisationPolicy)
		wantErr       bool
		expectedError error
	}{
		"should_fail_when_user_is_not_a_member_of_the_organisation": {
			authCtx: &auth.AuthenticatedUser{
				User: &datastore.User{
					UID: "randomstring",
				},
			},
			organisation: &datastore.Organisation{
				UID: "randomstring",
			},
			storeFn: func(orgP *OrganisationPolicy) {
				orgMem := orgP.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)

				orgMem.EXPECT().
					FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("Failed"))
			},
			wantErr:       true,
			expectedError: ErrNotAllowed,
		},
		"should_fail_when_user_is_not_a_super_user": {
			authCtx: &auth.AuthenticatedUser{
				User: &datastore.User{
					UID: "randomstring",
				},
			},
			organisation: &datastore.Organisation{
				UID: "randomstring",
			},
			storeFn: func(orgP *OrganisationPolicy) {
				orgMem := orgP.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)

				orgMem.EXPECT().
					FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.OrganisationMember{
						Role: auth.Role{Type: auth.RoleAPI},
					}, nil)
			},
			wantErr:       true,
			expectedError: ErrNotAllowed,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Arrange.
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			policy := &OrganisationPolicy{
				orgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
			}
			authCtx := context.WithValue(context.Background(), AuthCtxKey, tc.authCtx)

			if tc.storeFn != nil {
				tc.storeFn(policy)
			}

			// Act.
			err := policy.Update(authCtx, tc.organisation)

			// Assert.
			if tc.wantErr {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_OrganisationPolicy_Delete(t *testing.T) {
	tests := map[string]struct {
		authCtx       *auth.AuthenticatedUser
		organisation  *datastore.Organisation
		storeFn       func(*OrganisationPolicy)
		wantErr       bool
		expectedError error
	}{
		"should_fail_when_user_is_not_a_member_of_the_organisation": {
			authCtx: &auth.AuthenticatedUser{
				User: &datastore.User{
					UID: "randomstring",
				},
			},
			organisation: &datastore.Organisation{
				UID: "randomstring",
			},
			storeFn: func(orgP *OrganisationPolicy) {
				orgMem := orgP.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)

				orgMem.EXPECT().
					FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("Failed"))
			},
			wantErr:       true,
			expectedError: ErrNotAllowed,
		},
		"should_fail_when_user_is_not_a_super_user": {
			authCtx: &auth.AuthenticatedUser{
				User: &datastore.User{
					UID: "randomstring",
				},
			},
			organisation: &datastore.Organisation{
				UID: "randomstring",
			},
			storeFn: func(orgP *OrganisationPolicy) {
				orgMem := orgP.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)

				orgMem.EXPECT().
					FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.OrganisationMember{
						Role: auth.Role{Type: auth.RoleAPI},
					}, nil)
			},
			wantErr:       true,
			expectedError: ErrNotAllowed,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Arrange.
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			policy := &OrganisationPolicy{
				orgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
			}
			authCtx := context.WithValue(context.Background(), AuthCtxKey, tc.authCtx)

			if tc.storeFn != nil {
				tc.storeFn(policy)
			}

			// Act.
			err := policy.Delete(authCtx, tc.organisation)

			// Assert.
			if tc.wantErr {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}
