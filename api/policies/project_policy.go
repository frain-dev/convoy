package policies

import (
	"context"
	"errors"

	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

type ProjectPolicy struct {
	*authz.BasePolicy
	OrganisationRepo       datastore.OrganisationRepository
	OrganisationMemberRepo datastore.OrganisationMemberRepository
}

func (pp *ProjectPolicy) Manage(ctx context.Context, res interface{}) error {
	authCtx := ctx.Value(AuthUserCtx).(*auth.AuthenticatedUser)

	project, ok := res.(*datastore.Project)
	if !ok {
		return errors.New("Wrong project type")
	}

	org, err := pp.OrganisationRepo.FetchOrganisationByID(ctx, project.OrganisationID)
	if err != nil {
		return ErrNotAllowed
	}

	// API Access.

	apiKey, ok := authCtx.APIKey.(*datastore.APIKey)
	if ok {
		// API Key
		if apiKey.Role.Project != project.UID {
			return ErrNotAllowed
		}

		return nil
	}

	// Dashboard Access or Personal Access Token

	user, ok := authCtx.User.(*datastore.User)
	if !ok {
		return ErrNotAllowed
	}

	member, err := pp.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
	if err != nil {
		return ErrNotAllowed
	}

	if isSuperAdmin(member) || isAdmin(member) {
		return nil
	}

	return ErrNotAllowed
}

func (pp *ProjectPolicy) GetName() string {
	return "project"
}

func isAdmin(m *datastore.OrganisationMember) bool {
	return m.Role.Type == auth.RoleAdmin
}

func isSuperAdmin(m *datastore.OrganisationMember) bool {
	return m.Role.Type == auth.RoleSuperUser
}
