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

func Test_ProjectPolicy_Create(t *testing.T) {
	type test struct {
		basetest
		organisation *datastore.Organisation
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
				storeFn: func(pp *ProjectPolicy) {
					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
				storeFn: func(pp *ProjectPolicy) {
					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
				storeFn: func(pp *ProjectPolicy) {
					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
				storeFn: func(pp *ProjectPolicy) {
					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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

					opts := &ProjectPolicyOpts{
						OrganisationRepo:       mocks.NewMockOrganisationRepository(ctrl),
						OrganisationMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
					}
					policy := &ProjectPolicy{
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

func Test_ProjectPolicy_Read(t *testing.T) {
}

func Test_ProjectPolicy_Update(t *testing.T) {
	type test struct {
		basetest
		project *datastore.Project
		storeFn func(*ProjectPolicy)
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
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				project: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)
				},
			},
			{
				basetest: basetest{
					name: "should_allow_when_apikey_has_access_to_project",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID:         "randomstring",
							RoleProject: "project-uid",
						},
					},
					wantErr:       false,
					expectedError: nil,
				},
				project: &datastore.Project{
					UID: "project-uid",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

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
				project: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
				project: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
				project: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
				project: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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

					opts := &ProjectPolicyOpts{
						OrganisationRepo:       mocks.NewMockOrganisationRepository(ctrl),
						OrganisationMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
					}
					policy := &ProjectPolicy{
						opts: opts,
					}
					authCtx := context.WithValue(context.Background(), AuthCtxKey, tc.authCtx)

					if tc.storeFn != nil {
						tc.storeFn(policy)
					}

					// Act.
					err := policy.Update(authCtx, tc.project)

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

func Test_ProjectPolicy_Delete(t *testing.T) {
	type test struct {
		basetest
		project *datastore.Project
		storeFn func(*ProjectPolicy)
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
					wantErr:       true,
					expectedError: ErrNotAllowed,
				},
				project: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)
				},
			},
			{
				basetest: basetest{
					name: "should_allow_when_apikey_has_access_to_project",
					authCtx: &auth.AuthenticatedUser{
						APIKey: &datastore.APIKey{
							UID:         "randomstring",
							RoleProject: "project-uid",
						},
					},
					wantErr:       false,
					expectedError: nil,
				},
				project: &datastore.Project{
					UID: "project-uid",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

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
				project: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
				project: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
				project: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
				project: &datastore.Project{
					UID: "randomstring",
				},
				storeFn: func(pp *ProjectPolicy) {
					orgRepo := pp.opts.OrganisationRepo.(*mocks.MockOrganisationRepository)

					orgRepo.EXPECT().
						FetchOrganisationByID(gomock.Any(), gomock.Any()).
						Return(&datastore.Organisation{UID: "randomstring"}, nil)

					orgMemberRepo := pp.opts.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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

					opts := &ProjectPolicyOpts{
						OrganisationRepo:       mocks.NewMockOrganisationRepository(ctrl),
						OrganisationMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
					}
					policy := &ProjectPolicy{
						opts: opts,
					}

					authCtx := context.WithValue(context.Background(), AuthCtxKey, tc.authCtx)

					if tc.storeFn != nil {
						tc.storeFn(policy)
					}

					// Act.
					err := policy.Delete(authCtx, tc.project)

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
