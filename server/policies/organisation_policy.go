package policies

import (
	"context"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

type OrganisationPolicy struct {
	opts *OrganisationPolicyOpts
}

type OrganisationPolicyOpts struct {
	OrganisationMemberRepo datastore.OrganisationMemberRepository
}

func NewOrganisationPolicy(opts *OrganisationPolicyOpts) *OrganisationPolicy {
	return &OrganisationPolicy{
		opts: opts,
	}
}

func (op *OrganisationPolicy) Get(ctx context.Context, org *datastore.Organisation) error {
	authCtx := ctx.Value(AuthCtxKey).(*auth.AuthenticatedUser)

	user, ok := authCtx.User.(*datastore.User)
	if !ok {
		return ErrNotAllowed
	}

	member, err := op.opts.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
	if err != nil {
		return ErrNotAllowed
	}

	if member.Role.Type != auth.RoleSuperUser {
		return ErrNotAllowed
	}

	return nil
}

func (op *OrganisationPolicy) Update(ctx context.Context, org *datastore.Organisation) error {
	authCtx := ctx.Value(AuthCtxKey).(*auth.AuthenticatedUser)

	user, ok := authCtx.User.(*datastore.User)
	if !ok {
		return ErrNotAllowed
	}

	member, err := op.opts.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
	if err != nil {
		return ErrNotAllowed
	}

	if member.Role.Type != auth.RoleSuperUser {
		return ErrNotAllowed
	}

	return nil
}

func (op *OrganisationPolicy) Delete(ctx context.Context, org *datastore.Organisation) error {
	authCtx := ctx.Value(AuthCtxKey).(*auth.AuthenticatedUser)

	user, ok := authCtx.User.(*datastore.User)
	if !ok {
		return ErrNotAllowed
	}

	member, err := op.opts.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
	if err != nil {
		return ErrNotAllowed
	}

	if member.Role.Type != auth.RoleSuperUser {
		return ErrNotAllowed
	}

	return nil
}
