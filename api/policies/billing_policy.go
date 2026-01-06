package policies

import (
	"context"
	"errors"

	authz "github.com/Subomi/go-authz"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

type BillingPolicy struct {
	*authz.BasePolicy
	OrganisationMemberRepo datastore.OrganisationMemberRepository
}

func (bp *BillingPolicy) Manage(ctx context.Context, res interface{}) error {
	authCtx := ctx.Value(convoy.AuthUserCtx).(*auth.AuthenticatedUser)

	org, ok := res.(*datastore.Organisation)
	if !ok {
		return errors.New("wrong organisation type")
	}

	user, ok := authCtx.User.(*datastore.User)
	if !ok {
		return ErrNotAllowed
	}

	member, err := bp.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
	if err != nil {
		m, err := bp.OrganisationMemberRepo.FetchInstanceAdminByUserID(ctx, user.UID)
		if err == nil && isInstanceAdmin(m) {
			return nil
		}

		return ErrNotAllowed
	}

	// Allow billing admin or organisation admin to access billing
	if !isBillingAdmin(member) {
		return ErrNotAllowed
	}

	return nil
}

func (bp *BillingPolicy) GetName() string {
	return "billing"
}

// isBillingAdmin checks if the member is a billing admin or organisation admin
// Organisation admins have access to billing as they can manage the entire organisation
func isBillingAdmin(m *datastore.OrganisationMember) bool {
	return m.Role.Type == auth.RoleBillingAdmin || isOrganisationAdmin(m)
}
