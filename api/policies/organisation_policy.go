package policies

import (
	"context"
	"errors"

	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

type OrganisationPolicy struct {
	*authz.BasePolicy
	OrganisationMemberRepo datastore.OrganisationMemberRepository
}

func (op *OrganisationPolicy) Get(ctx context.Context, res interface{}) error {
	authCtx := ctx.Value(AuthCtxKey).(*auth.AuthenticatedUser)

	user, ok := authCtx.User.(*datastore.User)
	if !ok {
		return ErrNotAllowed
	}

	org, ok := res.(*datastore.Organisation)
	if !ok {
		return errors.New("Wrong organisation type")
	}

	member, err := op.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
	if err != nil {
		return ErrNotAllowed
	}

	if member.Role.Type != auth.RoleSuperUser {
		return ErrNotAllowed
	}

	return nil
}

func (op *OrganisationPolicy) Update(ctx context.Context, res interface{}) error {
	authCtx := ctx.Value(AuthCtxKey).(*auth.AuthenticatedUser)

	user, ok := authCtx.User.(*datastore.User)
	if !ok {
		return ErrNotAllowed
	}

	org, ok := res.(*datastore.Organisation)
	if !ok {
		return errors.New("Wrong organisation type")
	}

	member, err := op.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
	if err != nil {
		return ErrNotAllowed
	}

	if member.Role.Type != auth.RoleSuperUser {
		return ErrNotAllowed
	}

	return nil
}

func (op *OrganisationPolicy) Delete(ctx context.Context, res interface{}) error {
	authCtx := ctx.Value(AuthCtxKey).(*auth.AuthenticatedUser)

	user, ok := authCtx.User.(*datastore.User)
	if !ok {
		return ErrNotAllowed
	}

	org, ok := res.(*datastore.Organisation)
	if !ok {
		return errors.New("Wrong organisation type")
	}

	member, err := op.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
	if err != nil {
		return ErrNotAllowed
	}

	if member.Role.Type != auth.RoleSuperUser {
		return ErrNotAllowed
	}

	return nil
}

func (op *OrganisationPolicy) GetName() string {
	return "organisation"
}
