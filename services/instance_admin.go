package services

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
)

// IsPrimaryInstanceAdmin reports whether a user may access the instance under the
// current license. In multi-user mode (a valid seat-bearing license) every user
// passes. Otherwise only the primary instance admin (or, when no instance admin
// exists, the first org admin or the sole user) passes.
//
// This is the shared gate for the login and token-refresh paths so both enforce
// the same single-user-mode rule; a token issued before a license lapsed must not
// keep refreshing into full access.
func IsPrimaryInstanceAdmin(ctx context.Context, licenser license.Licenser, orgMemberRepo datastore.OrganisationMemberRepository, userRepo datastore.UserRepository, userID string) (bool, error) {
	// Check if multi-user mode is enabled (user_limit > 1)
	// MultiPlayerMode is redundant - user limits handle this
	isMultiUser, err := licenser.IsMultiUserMode(ctx)
	if err != nil {
		// Propagate the evaluation error instead of falling through to the
		// single-user admin checks. Swallowing it would let a transient
		// license/cache failure be reported to the caller as a definitive
		// "license expired" denial rather than a retryable server error.
		return false, err
	}
	if isMultiUser {
		// If multi-user mode, all users can access
		return true, nil
	}

	// Check if there are any instance admins
	count, err := orgMemberRepo.CountInstanceAdminUsers(ctx)
	if err != nil {
		return false, err
	}

	// If no instance admins exist, check if user is first org admin in any of their organisations
	if count == 0 {
		isFirstOrgAdmin, err := isFirstOrgAdminInAnyOrg(ctx, orgMemberRepo, userID)
		if err != nil {
			return false, err
		}

		// If user is not an org admin, check if they're the only user in the system
		if !isFirstOrgAdmin {
			userCount, err := userRepo.CountUsers(ctx)
			if err != nil {
				return false, err
			}
			if userCount == 1 {
				return true, nil
			}
		}

		return isFirstOrgAdmin, nil
	}

	// If instance admins exist, check if user is first instance admin
	return orgMemberRepo.IsFirstInstanceAdmin(ctx, userID)
}

func isFirstOrgAdminInAnyOrg(ctx context.Context, orgMemberRepo datastore.OrganisationMemberRepository, userID string) (bool, error) {
	// Get user's organisations
	orgs, _, err := orgMemberRepo.LoadUserOrganisationsPaged(ctx, userID, datastore.Pageable{
		PerPage: 100, // Get all orgs (reasonable limit)
	})
	if err != nil {
		return false, err
	}

	// Check if user is first org admin in any of their organisations
	for _, org := range orgs {
		member, err := orgMemberRepo.FetchOrganisationMemberByUserID(ctx, userID, org.UID)
		if err != nil {
			if errors.Is(err, datastore.ErrOrgMemberNotFound) {
				continue
			}
			return false, err
		}

		// Check if user is org admin
		if member.Role.Type != auth.RoleOrganisationAdmin {
			continue
		}

		// Get all org admins for this organisation to check if this user is the first
		members, _, err := orgMemberRepo.LoadOrganisationMembersPaged(ctx, org.UID, "", datastore.Pageable{
			PerPage: 100,
		})
		if err != nil {
			return false, err
		}

		// Find the first org admin by created_at
		var firstOrgAdmin *datastore.OrganisationMember
		for _, m := range members {
			if m.Role.Type == auth.RoleOrganisationAdmin {
				if firstOrgAdmin == nil || m.CreatedAt.Before(firstOrgAdmin.CreatedAt) {
					firstOrgAdmin = m
				}
			}
		}

		// If this user is the first org admin in this org, allow access
		if firstOrgAdmin != nil && firstOrgAdmin.UserID == userID {
			return true, nil
		}
	}

	return false, nil
}
