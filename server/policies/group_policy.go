package policies

import (
	"context"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

type GroupPolicy struct {
	orgRepo       datastore.OrganisationRepository
	orgMemberRepo datastore.OrganisationMemberRepository
	gRepo         datastore.GroupRepository
}

func (gp *GroupPolicy) Get(ctx context.Context, group datastore.Group) error {
	authCtx := ctx.Value(AuthCtxKey).(auth.AuthenticatedUser)

	org, err := gp.orgRepo.FetchOrganisationByID(ctx, group.OrganisationID)
	if err != nil {
		return ErrNotAllowed
	}

	// Personal Access Tokens.
	if authCtx.User != nil {
		_, err := gp.orgMemberRepo.FetchOrganisationMemberByUserID(ctx, authCtx.APIKey.UserID, org.UID)
		if err != nil {
			return ErrNotAllowed
		}

		return nil
	}

	// JWT Access.
	orgPolicy := OrganisationPolicy{gp.orgMemberRepo}
	return orgPolicy.Get(ctx, org)
}

func (gp *GroupPolicy) Create(ctx context.Context, org *datastore.Organisation) error {
	authCtx := ctx.Value(AuthCtxKey).(auth.AuthenticatedUser)

	// Personal Access Tokens.
	if authCtx.User != nil {
		_, err := gp.orgMemberRepo.FetchOrganisationMemberByUserID(ctx, authCtx.APIKey.UserID, org.UID)
		if err != nil {
			return ErrNotAllowed
		}

		return nil
	}

	// JWT Access
	orgPolicy := OrganisationPolicy{gp.orgMemberRepo}
	return orgPolicy.Get(ctx, org)
}

func (gp *GroupPolicy) Update(ctx context.Context, group datastore.Group) error {
	return gp.Get(ctx, group)
}

func (gp *GroupPolicy) Delete(ctx context.Context, group datastore.Group) error {
	return gp.Get(ctx, group)
}
