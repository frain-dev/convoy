package policies

import (
	"context"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

type ProjectPolicy struct {
	opts *ProjectPolicyOpts
}

type ProjectPolicyOpts struct {
	OrganisationRepo       datastore.OrganisationRepository
	OrganisationMemberRepo datastore.OrganisationMemberRepository
}

func NewProjectPolicy(opts *ProjectPolicyOpts) *ProjectPolicy {
	return &ProjectPolicy{
		opts: opts,
	}
}

func (pp *ProjectPolicy) Get(ctx context.Context, project *datastore.Project) error {
	authCtx := ctx.Value(AuthCtxKey).(*auth.AuthenticatedUser)

	org, err := pp.opts.OrganisationRepo.FetchOrganisationByID(ctx, project.OrganisationID)
	if err != nil {
		return ErrNotAllowed
	}

	apiKey, ok := authCtx.APIKey.(*datastore.APIKey)
	if ok {
		// Personal Access Tokens
		if apiKey.Type == datastore.PersonalKey {
			_, err := pp.opts.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, apiKey.UserID, org.UID)
			if err != nil {
				return ErrNotAllowed
			}

			return nil
		}

		// API Key
		if apiKey.RoleProject != project.UID {
			return ErrNotAllowed
		}

		return nil
	}

	// JWT Access.
	opts := &OrganisationPolicyOpts{
		OrganisationMemberRepo: pp.opts.OrganisationMemberRepo,
	}
	orgPolicy := OrganisationPolicy{opts}
	return orgPolicy.Get(ctx, org)
}

func (pp *ProjectPolicy) Create(ctx context.Context, org *datastore.Organisation) error {
	authCtx := ctx.Value(AuthCtxKey).(*auth.AuthenticatedUser)

	apiKey, ok := authCtx.APIKey.(*datastore.APIKey)
	if ok {
		// Personal Access Tokens.
		if apiKey.Type == datastore.PersonalKey {
			_, err := pp.opts.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, apiKey.UserID, org.UID)
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
		OrganisationMemberRepo: pp.opts.OrganisationMemberRepo,
	}
	orgPolicy := OrganisationPolicy{opts}
	return orgPolicy.Get(ctx, org)
}

func (pp *ProjectPolicy) Update(ctx context.Context, project *datastore.Project) error {
	return pp.Get(ctx, project)
}

func (pp *ProjectPolicy) Delete(ctx context.Context, project *datastore.Project) error {
	return pp.Get(ctx, project)
}
