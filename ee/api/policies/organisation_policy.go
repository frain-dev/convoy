package policies

import (
	"context"
	"errors"

	authz "github.com/Subomi/go-authz"
	basepolicy "github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

type OrganisationPolicy struct {
	*authz.BasePolicy
}

func (op *OrganisationPolicy) Manage(ctx context.Context, res interface{}) error {
	authCtx := ctx.Value(basepolicy.AuthCtxKey).(*auth.AuthenticatedUser)

	org, ok := res.(*datastore.Organisation
	if !ok {
		return errors.New("Wrong organisation type")
	}

	// Dashboard Access or Personal Access Token

	user, ok := authCtx.User.(*datastore.User)
	if !ok {
		return basepolicy.ErrNotAllowed
	}

	member, err := pp.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
	if err != nil {
		return basepolicy.ErrNotAllowed
	}

	if isAllowed := isSuperAdmin(member); !isAllowed {
		return basepolicy.ErrNotAllowed
	}

	return nil
}
