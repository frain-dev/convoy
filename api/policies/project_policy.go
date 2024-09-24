package policies

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/internal/pkg/license"

	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

type ProjectPolicy struct {
	*authz.BasePolicy
	OrganisationRepo       datastore.OrganisationRepository
	OrganisationMemberRepo datastore.OrganisationMemberRepository
	Licenser               license.Licenser
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

	// Dashboard Access or Personal Access Token
	if authCtx.User != nil {
		user, ok := authCtx.User.(*datastore.User)
		if !ok {
			return ErrNotAllowed
		}
		member, err := pp.OrganisationMemberRepo.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
		if err != nil {
			return ErrNotAllowed
		}

		// to allow admin roles, MultiPlayerMode must be enabled
		adminAllowed := isAdmin(member) && pp.Licenser.MultiPlayerMode()

		if isSuperAdmin(member) || adminAllowed {
			return nil
		}

		return ErrNotAllowed
	}

	// API Key Access.
	apiKey, ok := authCtx.APIKey.(*datastore.APIKey)
	if !ok {
		return ErrNotAllowed
	}

	if apiKey.Role.Project != project.UID {
		return ErrNotAllowed
	}
	return nil
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
