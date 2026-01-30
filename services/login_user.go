package services

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
)

type LoginUserService struct {
	UserRepo      datastore.UserRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	Cache         cache.Cache
	JWT           *jwt.Jwt
	Data          *models.LoginUser
	Licenser      license.Licenser
}

func (u *LoginUserService) isPrimaryInstanceAdmin(ctx context.Context, userID string) (bool, error) {
	// Check if multi-user mode is enabled (user_limit > 1)
	// MultiPlayerMode is redundant - user limits handle this
	isMultiUser, err := u.Licenser.IsMultiUserMode(ctx)
	if err == nil && isMultiUser {
		// If multi-user mode, all users can access
		return true, nil
	}

	// Check if there are any instance admins
	count, err := u.OrgMemberRepo.CountInstanceAdminUsers(ctx)
	if err != nil {
		return false, err
	}

	// If no instance admins exist, check if user is first org admin in any of their organisations
	if count == 0 {
		isFirstOrgAdmin, err := u.isFirstOrgAdminInAnyOrg(ctx, userID)
		if err != nil {
			return false, err
		}

		// If user is not an org admin, check if they're the only user in the system
		if !isFirstOrgAdmin {
			userCount, err := u.UserRepo.CountUsers(ctx)
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
	isFirst, err := u.OrgMemberRepo.IsFirstInstanceAdmin(ctx, userID)
	if err != nil {
		return false, err
	}

	return isFirst, nil
}

func (u *LoginUserService) isFirstOrgAdminInAnyOrg(ctx context.Context, userID string) (bool, error) {
	// Get user's organisations
	orgs, _, err := u.OrgMemberRepo.LoadUserOrganisationsPaged(ctx, userID, datastore.Pageable{
		PerPage: 100, // Get all orgs (reasonable limit)
	})
	if err != nil {
		return false, err
	}

	// Check if user is first org admin in any of their organisations
	for _, org := range orgs {
		member, err := u.OrgMemberRepo.FetchOrganisationMemberByUserID(ctx, userID, org.UID)
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
		members, _, err := u.OrgMemberRepo.LoadOrganisationMembersPaged(ctx, org.UID, "", datastore.Pageable{
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

func (u *LoginUserService) Run(ctx context.Context) (*datastore.User, *jwt.Token, error) {
	user, err := u.UserRepo.FindUserByEmail(ctx, u.Data.Username)
	if err != nil {
		if errors.Is(err, datastore.ErrUserNotFound) {
			return nil, nil, &ServiceError{ErrMsg: "invalid username or password", Err: err}
		}

		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}

	p := datastore.Password{Plaintext: u.Data.Password, Hash: []byte(user.Password)}
	match, err := p.Matches()
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}
	if !match {
		return nil, nil, &ServiceError{ErrMsg: "invalid username or password"}
	}

	// Check if user can access based on license status and get instance admin count
	canAccess, err := u.isPrimaryInstanceAdmin(ctx, user.UID)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}

	if !canAccess {
		return nil, nil, &ServiceError{
			Code:   ErrCodeLicenseExpired,
			ErrMsg: "License expired. Only the first organization administrator can access the system"}
	}

	token, err := u.JWT.GenerateToken(user)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}

	return user, &token, nil
}
