package policies

import (
	"context"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

type GroupPolicy struct {
	opts *GroupPolicyOpts
}

type GroupPolicyOpts struct {
	OrganisationRepo       datastore.OrganisationRepository
	OrganisationMemberRepo datastore.OrganisationMemberRepository
}

func NewGroupPolicy(opts *GroupPolicyOpts) *GroupPolicy {
	return &GroupPolicy{
		opts: opts,
	}
}

func (gp *GroupPolicy) Get(ctx context.Context, group *datastore.Group) error {
	authCtx := ctx.Value(AuthCtxKey).(*auth.AuthenticatedUser)

	org, err := gp.opts.OrganisationRepo.FetchOrganisationByID(ctx, group.OrganisationID)
	if err != nil {
		return ErrNotAllowed
	}

	apiKey, ok := authCtx.APIKey.(*datastore.APIKey)
	if ok {
		// Personal Access Tokens
		if apiKey.Type == datastore.PersonalKey {
			_, err := gp.opts.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, apiKey.UserID, org.UID)
			if err != nil {
				return ErrNotAllowed
			}

			return nil
		}

		// API Key
		if apiKey.Role.Group != group.UID {
			return ErrNotAllowed
		}

		return nil
	}

	// JWT Access.
	opts := &OrganisationPolicyOpts{
		OrganisationMemberRepo: gp.opts.OrganisationMemberRepo,
	}
	orgPolicy := OrganisationPolicy{opts}
	return orgPolicy.Get(ctx, org)
}

func (gp *GroupPolicy) Create(ctx context.Context, org *datastore.Organisation) error {
	authCtx := ctx.Value(AuthCtxKey).(*auth.AuthenticatedUser)

	apiKey, ok := authCtx.APIKey.(*datastore.APIKey)
	if ok {
		// Personal Access Tokens.
		if apiKey.Type == datastore.PersonalKey {
			_, err := gp.opts.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, apiKey.UserID, org.UID)
			if err != nil {
				return ErrNotAllowed
			}

			return nil
		}

		// API Key
		return ErrNotAllowed
	}

	// JWT Access
	opts := &OrganisationPolicyOpts{
		OrganisationMemberRepo: gp.opts.OrganisationMemberRepo,
	}
	orgPolicy := OrganisationPolicy{opts}
	return orgPolicy.Get(ctx, org)
}

func (gp *GroupPolicy) Update(ctx context.Context, group *datastore.Group) error {
	return gp.Get(ctx, group)
}

func (gp *GroupPolicy) Delete(ctx context.Context, group *datastore.Group) error {
	return gp.Get(ctx, group)
}
