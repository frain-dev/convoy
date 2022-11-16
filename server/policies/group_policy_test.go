package policies

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func Test_GroupPolicy_Create(t *testing.T) {
	type test struct {
		name    string
		wantErr bool
	}

	testmatrix := map[string][]test{
		"project_api_keys": []test{
			{
				wantErr: false,
			},
			{
				wantErr: false,
			},
		},
	}

	for name, tests := range testmatrix {
		t.Run(name, func(t *testing.T) {
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					fmt.Println("Oporrrr", tc)
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

	testmatrix := map[string]test{
		"personal_api_key": {},
	}

	for name, authCtx := range testmatrix {
		t.Run(name, func(t *testing.T) {
			for tname, tc := range tests {
				t.Run(tname, func(t *testing.T) {
					fmt.Println("Oporrr", tc, authCtx)
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
		"project_api_key": []test{
			{
				name: "should_reject_when_apikey_does_not_have_access_to_group",
				authCtx: &auth.AuthenticatedUser{
					APIKey: &datastore.APIKey{
						UID: "randomstring",
					},
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.orgRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)
				},
				wantErr:       true,
				expectedError: ErrNotAllowed,
			},
			{
				name: "should_allow_when_apikey_has_access_to_group",
				authCtx: &auth.AuthenticatedUser{
					APIKey: &datastore.APIKey{
						UID: "randomstring",
						Role: auth.Role{
							Group: "group-uid",
						},
					},
				},
				group: &datastore.Group{
					UID: "group-uid",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.orgRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)
				},
				wantErr:       false,
				expectedError: nil,
			},
		},
		"personal_api_key": []test{
			{
				name: "should_reject_when_user_does_not_belong_to_organisation",
				authCtx: &auth.AuthenticatedUser{
					APIKey: &datastore.APIKey{
						UID:  "randomstring",
						Type: datastore.PersonalKey,
					},
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.orgRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("rejected"))
				},
				wantErr:       true,
				expectedError: ErrNotAllowed,
			},
			{
				name: "should_allow_when_user_does_not_belong_to_organisation",
				authCtx: &auth.AuthenticatedUser{
					APIKey: &datastore.APIKey{
						UID:  "randomstring",
						Type: datastore.PersonalKey,
					},
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.orgRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&datastore.OrganisationMember{UID: "randomstring"}, nil)
				},
				wantErr:       false,
				expectedError: nil,
			},
		},
		"user": []test{
			{
				name: "should_reject_when_user_does_not_belong_to_organisation",
				authCtx: &auth.AuthenticatedUser{
					User: &datastore.User{
						UID: "randomstring",
					},
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.orgRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("rejected"))
				},
				wantErr:       true,
				expectedError: ErrNotAllowed,
			},
			{
				name: "should_allow_when_user_belong_to_organisation",
				authCtx: &auth.AuthenticatedUser{
					User: &datastore.User{
						UID: "randomstring",
					},
				},
				group: &datastore.Group{
					UID: "randomstring",
				},
				storeFn: func(gp *GroupPolicy) {
					orgRepo := gp.orgRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := gp.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)

					orgMemberRepo.EXPECT().
						FetchOrganisationMemberByUserID(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(&datastore.OrganisationMember{
							UID: "randomstring",
							Role: auth.Role{
								Type: auth.RoleSuperUser,
							},
						}, nil)
				},
				wantErr:       false,
				expectedError: nil,
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

					policy := &GroupPolicy{
						orgRepo:       mocks.NewMockOrganisationRepository(ctrl),
						orgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
						gRepo:         mocks.NewMockGroupRepository(ctrl),
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
