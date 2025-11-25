package policies

import (
	"context"
	"errors"

	authz "github.com/Subomi/go-authz"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
)

type ProjectPolicy struct {
	*authz.BasePolicy
	OrganisationRepo       datastore.OrganisationRepository
	OrganisationMemberRepo datastore.OrganisationMemberRepository
	Licenser               license.Licenser
}

func (pp *ProjectPolicy) Manage(ctx context.Context, res interface{}) error {
	return pp.checkAccess(ctx, res, func(member *datastore.OrganisationMember) bool {
		adminAllowed := isProjectAdmin(member) && pp.Licenser.MultiPlayerMode()
		return isOrganisationAdmin(member) || adminAllowed
	})
}

func (pp *ProjectPolicy) View(ctx context.Context, res interface{}) error {
	return pp.checkAccess(ctx, res, func(member *datastore.OrganisationMember) bool {
		viewerAllowed := isProjectViewer(member) && pp.Licenser.MultiPlayerMode()
		return isOrganisationAdmin(member) || viewerAllowed
	})
}

func (pp *ProjectPolicy) checkAccess(ctx context.Context, res interface{}, checkMember func(*datastore.OrganisationMember) bool) error {
	authCtx := ctx.Value(AuthUserCtx).(*auth.AuthenticatedUser)

	project, ok := res.(*datastore.Project)
	if !ok {
		return errors.New("wrong project type")
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
			m, err := pp.OrganisationMemberRepo.FetchInstanceAdminByUserID(ctx, user.UID)
			if err == nil && isInstanceAdmin(m) {
				return nil
			}
			return ErrNotAllowed
		}

		if checkMember(member) {
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

func isProjectViewer(m *datastore.OrganisationMember) bool {
	return m.Role.Type == auth.RoleProjectViewer || isProjectAdmin(m)
}

func isProjectAdmin(m *datastore.OrganisationMember) bool {
	return m.Role.Type == auth.RoleProjectAdmin || isOrganisationAdmin(m)
}

func isOrganisationAdmin(m *datastore.OrganisationMember) bool {
	return m.Role.Type == auth.RoleOrganisationAdmin || isInstanceAdmin(m)
}

func isInstanceAdmin(m *datastore.OrganisationMember) bool {
	return m.Role.Type == auth.RoleInstanceAdmin
}
