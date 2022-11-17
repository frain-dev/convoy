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

func Test_GroupPolicy_Create(t *testing.T) {
	type test struct {
		basetest
		organisation *datastore.Organisation
		storeFn      func(*GroupPolicy)
	}

	testmatrix := map[string][]test{
		"project_api_key": {
			{
				basetest: basetest{
					name: "should_reject_when_apikey_does_not_have_access_to_group",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID: "randomstring",
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
			},
		},
		"personal_api_key": {
			{
				basetest: basetest{
					name: "should_reject_when_user_does_not_belong_to_organisation",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID:  "randomstring",
							Type: datastore.PersonalKey,
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("rejected"))
				},
			},
			{
				basetest: basetest{
					name: "should_allow_when_user_does_not_belong_to_organisation",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID:  "randomstring",
							Type: datastore.PersonalKey,
						},
					},
					wantErr:       false,
					expectedError: nil,
				},
				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&datastore.OrganisationMember{UID: "randomstring"}, nil)
				},
			},
		},
		"user": {
			{
				basetest: basetest{
					name: "should_reject_when_user_does_not_belong_to_organisation",
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
				storeFn: func(gp *GroupPolicy) {
					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("rejected"))
				},
			},
			{
				basetest: basetest{
					name: "should_allow_when_user_belong_to_organisation",
					authCtx: &auth.AuthenticatedUser{
						User: &datastore.User{
							UID: "randomstring",
						},
					},
					wantErr:       false,
					expectedError: nil,
				},
				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&datastore.OrganisationMember{
							UID: "randomstring",
							Role: auth.Role{
								Type: auth.RoleSuperUser,
							},
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

					opts := &GroupPolicyOpts{
						OrganisationRepo:       mocks.NewMockOrganisationRepository(ctrl),
						OrganisationMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
					}
					policy := &GroupPolicy{
						opts: opts,
					}
					authCtx := context.WithValue(context.Background(), AuthCtxKey, tc.authCtx)

					if tc.storeFn != nil {
						tc.storeFn(policy)
					}

					// Act.
					err := policy.Create(authCtx, tc.organisation)

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

func Test_GroupPolicy_Read(t *testing.T) {
}

func Test_GroupPolicy_Update(t *testing.T) {
	type test struct {
		basetest
		group   *datastore.Group
		storeFn func(*GroupPolicy)
	}

	testmatrix := map[string][]test{
		"project_api_key": {
			{
				basetest: basetest{
					name: "should_reject_when_apikey_does_not_have_access_to_group",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID: "randomstring",
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)
				},
			},
			{
				basetest: basetest{
					name: "should_allow_when_apikey_has_access_to_group",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID: "randomstring",
							Role: auth.Role{
								Group: "group-uid",
							},
						},
					},
					wantErr:       false,
					expectedError: nil,
				},
				group: &datastore.Group{
					UID: "group-uid",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)
				},
			},
		},
		"personal_api_key": {
			{
				basetest: basetest{
					name: "should_reject_when_user_does_not_belong_to_organisation",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID:  "randomstring",
							Type: datastore.PersonalKey,
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("rejected"))
				},
			},
			{
				basetest: basetest{
					name: "should_allow_when_user_does_not_belong_to_organisation",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID:  "randomstring",
							Type: datastore.PersonalKey,
						},
					},
					wantErr:       false,
					expectedError: nil,
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&datastore.OrganisationMember{UID: "randomstring"}, nil)
				},
			},
		},
		"user": {
			{
				basetest: basetest{
					name: "should_reject_when_user_does_not_belong_to_organisation",
					authCtx: &auth.AuthenticatedUser{
						User: &datastore.User{
							UID: "randomstring",
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("rejected"))
				},
			},
			{
				basetest: basetest{
					name: "should_allow_when_user_belong_to_organisation",
					authCtx: &auth.AuthenticatedUser{
						User: &datastore.User{
							UID: "randomstring",
						},
					},
					wantErr:       false,
					expectedError: nil,
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&datastore.OrganisationMember{
							UID: "randomstring",
							Role: auth.Role{
								Type: auth.RoleSuperUser,
							},
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

					opts := &GroupPolicyOpts{
						OrganisationRepo:       mocks.NewMockOrganisationRepository(ctrl),
						OrganisationMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
					}
					policy := &GroupPolicy{
						opts: opts,
					}
					authCtx := context.WithValue(context.Background(), AuthCtxKey, tc.authCtx)

					if tc.storeFn != nil {
						tc.storeFn(policy)
					}

					// Act.
					err := policy.Update(authCtx, tc.group)

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

func Test_GroupPolicy_Delete(t *testing.T) {
	type test struct {
		basetest
		group   *datastore.Group
		storeFn func(*GroupPolicy)
	}

	testmatrix := map[string][]test{
		"project_api_key": {
			{
				basetest: basetest{
					name: "should_reject_when_apikey_does_not_have_access_to_group",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID: "randomstring",
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)
				},
			},
			{
				basetest: basetest{
					name: "should_allow_when_apikey_has_access_to_group",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID: "randomstring",
							Role: auth.Role{
								Group: "group-uid",
							},
						},
					},
					wantErr:       false,
					expectedError: nil,
				},
				group: &datastore.Group{
					UID: "group-uid",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)
				},
			},
		},
		"personal_api_key": {
			{
				basetest: basetest{
					name: "should_reject_when_user_does_not_belong_to_organisation",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID:  "randomstring",
							Type: datastore.PersonalKey,
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("rejected"))
				},
			},
			{
				basetest: basetest{
					name: "should_allow_when_user_does_not_belong_to_organisation",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID:  "randomstring",
							Type: datastore.PersonalKey,
						},
					},
					wantErr:       false,
					expectedError: nil,
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&datastore.OrganisationMember{UID: "randomstring"}, nil)
				},
			},
		},
		"user": {
			{
				basetest: basetest{
					name: "should_reject_when_user_does_not_belong_to_organisation",
					authCtx: &auth.AuthenticatedUser{
						User: &datastore.User{
							UID: "randomstring",
						},
					},
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("rejected"))
				},
			},
			{
				basetest: basetest{
					name: "should_allow_when_user_belong_to_organisation",
					authCtx: &auth.AuthenticatedUser{
						User: &datastore.User{
							UID: "randomstring",
						},
					},
					wantErr:       false,
					expectedError: nil,
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&datastore.OrganisationMember{
							UID: "randomstring",
							Role: auth.Role{
								Type: auth.RoleSuperUser,
							},
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

					opts := &GroupPolicyOpts{
						OrganisationRepo:       mocks.NewMockOrganisationRepository(ctrl),
						OrganisationMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
					}
					policy := &GroupPolicy{
						opts: opts,
					}

					authCtx := context.WithValue(context.Background(), AuthCtxKey, tc.authCtx)

					if tc.storeFn != nil {
						tc.storeFn(policy)
					}

					// Act.
					err := policy.Delete(authCtx, tc.group)

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
