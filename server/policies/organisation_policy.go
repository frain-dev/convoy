package policies

import (
	"context"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

type OrganisationPolicy struct {
	orgMemberRepo datastore.OrganisationMemberRepository
}

func (op *OrganisationPolicy) Get(ctx context.Context, org *datastore.Organisation) error {
	authCtx := ctx.Value(AuthCtxKey).(auth.AuthenticatedUser)

	_, err := op.orgMemberRepo.FetchOrganisationMemberByUserID(ctx, authCtx.User.UID, org.UID)
	if err != nil {
		return ErrNotAllowed
	}

	return nil
}

func (op *OrganisationPolicy) Update(ctx context.Context, org *datastore.Organisation) error {
	authCtx := ctx.Value(AuthCtxKey).(auth.AuthenticatedUser)

	member, err := op.orgMemberRepo.FetchOrganisationMemberByUserID(ctx, authCtx.User.UID, org.UID)
	if err != nil {
		return ErrNotAllowed
	}

	if member.Role.Type != auth.RoleSuperUser {
		return ErrNotAllowed
	}

	return nil
}

func (op *OrganisationPolicy) Delete(ctx context.Context, org *datastore.Organisation) error {
	authCtx := ctx.Value(AuthCtxKey).(auth.AuthenticatedUser)

	member, err := op.orgMemberRepo.FetchOrganisationMemberByUserID(ctx, authCtx.User.UID, org.UID)
	if err != nil {
		return ErrNotAllowed
	}

	if member.Role.Type != auth.RoleSuperUser {
		return ErrNotAllowed
	}

	return nil
}
