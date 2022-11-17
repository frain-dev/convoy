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
	type test struct {
		basetest
		organisation *datastore.Organisation
		storeFn      func(*OrganisationPolicy)
	}

	testmatrix := map[string][]test{
		"user": {
			{
				basetest: basetest{
					name: "should_fail_when_user_is_not_a_member_of_the_organisation",
					authCtx: &auth.AuthenticatedUser{
						User: &datastore.User{
							UID: "randomstring",
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
				storeFn: func(orgP *OrganisationPolicy) {
					orgMem := orgP.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMem.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("Failed"))
				},
			},
			{
				basetest: basetest{
					name: "should_fail_when_user_is_not_a_super_user",
					authCtx: &auth.AuthenticatedUser{
						User: &datastore.User{
							UID: "randomstring",
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
				storeFn: func(orgP *OrganisationPolicy) {
					orgMem := orgP.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMem.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&datastore.OrganisationMember{
							Role: auth.Role{Type: auth.RoleAPI},
						}, nil)
				},
			},
		},
	}

	for name, test := range testmatrix {
		t.Run(name, func(t *testing.T) {
			for _, tc := range test {
				t.Run(tc.name, func(t *testing.T) {
					// Arrange.
					ctrl := gomock.NewController(t)
					defer ctrl.Finish()

					opts := &OrganisationPolicyOpts{
						OrganisationMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
					}
					policy := &OrganisationPolicy{opts}
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
		})
	}
}

func Test_OrganisationPolicy_Update(t *testing.T) {
	type test struct {
		basetest
		organisation *datastore.Organisation
		storeFn      func(*OrganisationPolicy)
	}

	testmatrix := map[string][]test{
		"user": {
			{
				basetest: basetest{
					name: "should_fail_when_user_is_not_a_member_of_the_organisation",
					authCtx: &auth.AuthenticatedUser{
						User: &datastore.User{
							UID: "randomstring",
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
				storeFn: func(orgP *OrganisationPolicy) {
					orgMem := orgP.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMem.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("Failed"))
				},
			},
			{
				basetest: basetest{
					name: "should_fail_when_user_is_not_a_super_user",
					authCtx: &auth.AuthenticatedUser{
						User: &datastore.User{
							UID: "randomstring",
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
				storeFn: func(orgP *OrganisationPolicy) {
					orgMem := orgP.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMem.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&datastore.OrganisationMember{
							Role: auth.Role{Type: auth.RoleAPI},
						}, nil)
				},
			},
		},
	}

	for name, test := range testmatrix {
		t.Run(name, func(t *testing.T) {
			for _, tc := range test {
				t.Run(tc.name, func(t *testing.T) {
					// Arrange.
					ctrl := gomock.NewController(t)
					defer ctrl.Finish()

					opts := &OrganisationPolicyOpts{
						OrganisationMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
					}
					policy := &OrganisationPolicy{opts}
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
		})
	}
}

func Test_OrganisationPolicy_Delete(t *testing.T) {
	type test struct {
		basetest
		organisation *datastore.Organisation
		storeFn      func(*OrganisationPolicy)
	}

	testmatrix := map[string][]test{
		"user": {
			{
				basetest: basetest{
					name: "should_fail_when_user_is_not_a_member_of_the_organisation",
					authCtx: &auth.AuthenticatedUser{
						User: &datastore.User{
							UID: "randomstring",
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
				storeFn: func(orgP *OrganisationPolicy) {
					orgMem := orgP.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMem.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("Failed"))
				},
			},
			{
				basetest: basetest{
					name: "should_fail_when_user_is_not_a_super_user",
					authCtx: &auth.AuthenticatedUser{
						User: &datastore.User{
							UID: "randomstring",
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},

				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
				storeFn: func(orgP *OrganisationPolicy) {
					orgMem := orgP.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMem.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&datastore.OrganisationMember{
							Role: auth.Role{Type: auth.RoleAPI},
						}, nil)
				},
			},
		},
	}

	for name, test := range testmatrix {
		t.Run(name, func(t *testing.T) {
			for _, tc := range test {
				t.Run(tc.name, func(t *testing.T) {
					// Arrange.
					ctrl := gomock.NewController(t)
					defer ctrl.Finish()

					opts := &OrganisationPolicyOpts{
						OrganisationMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
					}
					policy := &OrganisationPolicy{opts}
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
		})
	}
}
