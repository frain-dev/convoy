package policies

import (
	"context"
	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

type UserPolicy struct {
	*authz.BasePolicy
	OrganisationMemberRepo datastore.OrganisationMemberRepository
}

func (op *UserPolicy) GodMode(ctx context.Context, res interface{}) error {
	authCtx := ctx.Value(AuthUserCtx).(*auth.AuthenticatedUser)

	user, ok := authCtx.User.(*datastore.User)
	if !ok {
		return ErrNotAllowed
	}

	member, err := op.OrganisationMemberRepo.FetchAnyInstanceAdminOrRootByUserID(ctx, user.UID)
	if err != nil {
		return ErrNotAllowed
	}

	if !isInstanceAdmin(member) {
		return ErrNotAllowed
	}

	return nil
}

func (op *UserPolicy) GetName() string {
	return "user"
}
