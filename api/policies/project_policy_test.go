package policies

import (
	"context"
	"errors"
	"testing"

	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_ProjectPolicy_Manage(t *testing.T) {
	type test struct {
		basetest
		organisation *datastore.Project
		storeFn      func(*ProjectPolicy)
	}

	testmatrix := map[string][]test{
		"project_api_key": {
			{
				basetest: basetest{
					name: "should_reject_when_apikey_does_not_have_access_to_project",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID: "randomstring",
						},
					},
					assertion:     require.Error,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "123"}, nil)
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
					assertion:     require.Error,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "123"}, nil)

					orgMemberRepo := pp.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
					assertion:     require.NoError,
					expectedError: nil,
				},
				organisation: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "123"}, nil)

					orgMemberRepo := pp.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
					assertion:     require.Error,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "123"}, nil)

					orgMemberRepo := pp.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
					assertion:     require.NoError,
					expectedError: nil,
				},
				organisation: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "123"}, nil)

					orgMemberRepo := pp.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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

					policy := &ProjectPolicy{
						BasePolicy:             authz.NewBasePolicy(),
						OrganisationRepo:       mocks.NewMockOrganisationRepository(ctrl),
						OrganisationMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
					}

					policy.SetRule("manage", authz.RuleFunc(policy.Manage))

					if tc.storeFn != nil {
						tc.storeFn(policy)
					}

					ctx := context.WithValue(context.Background(), AuthUserCtx, tc.authCtx)

					az, _ := authz.NewAuthz(&authz.AuthzOpts{})
					_ = az.RegisterPolicy(policy)

					// Act.
					err := az.Authorize(ctx, "project.manage", tc.organisation)

					// Assert.
					tc.assertion(t, err)
				})
			}
		})
	}
}
