package policies

import (
	"context"
	"errors"
	"testing"

	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func Test_OrganisationPolicy_Manage(t *testing.T) {
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
					assertion:     require.Error,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
				storeFn: func(orgP *OrganisationPolicy) {
					orgMem := orgP.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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
					assertion:     require.Error,
					expectedError: ErrNotAllowed,
				},
				organisation: &datastore.Organisation{
					UID: "randomstring",
				},
				storeFn: func(orgP *OrganisationPolicy) {
					orgMem := orgP.OrganisationMemberRepo.(*mocks.MockOrganisationMemberRepository)

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

					policy := &OrganisationPolicy{
						BasePolicy:             authz.NewBasePolicy(),
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
					err := az.Authorize(ctx, "organisation.manage", tc.organisation)

					// Assert.
					tc.assertion(t, err)
				})
			}
		})
	}
}
