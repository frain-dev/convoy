package services

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
)

func PrimaryInstanceAccess(ctx context.Context, userID string, userRepo datastore.UserRepository, orgMemberRepo datastore.OrganisationMemberRepository, licenser license.Licenser) (bool, error) {
	isMultiUser, err := licenser.IsMultiUserMode(ctx)
	if err == nil && isMultiUser {
		return true, nil
	}

	count, err := orgMemberRepo.CountInstanceAdminUsers(ctx)
	if err != nil {
		return false, err
	}

	if count == 0 {
		isFirstOrgAdmin, err := firstOrgAdminInAnyOrg(ctx, userID, orgMemberRepo)
		if err != nil {
			return false, err
		}

		if !isFirstOrgAdmin {
			userCount, err := userRepo.CountUsers(ctx)
			if err != nil {
				return false, err
			}
			if userCount == 1 {
				return true, nil
			}

			isFirstOrphan, err := firstOrphanUser(ctx, userID, userRepo, orgMemberRepo)
			if err != nil {
				return false, err
			}
			if isFirstOrphan {
				return true, nil
			}
		}

		return isFirstOrgAdmin, nil
	}

	return orgMemberRepo.IsFirstInstanceAdmin(ctx, userID)
}

func firstOrphanUser(ctx context.Context, userID string, userRepo datastore.UserRepository, orgMemberRepo datastore.OrganisationMemberRepository) (bool, error) {
	orgs, _, err := orgMemberRepo.LoadUserOrganisationsPaged(ctx, userID, datastore.Pageable{PerPage: 1})
	if err != nil {
		return false, err
	}
	if len(orgs) > 0 {
		return false, nil
	}

	firstUser, err := userRepo.FindFirstUser(ctx)
	if err != nil {
		if errors.Is(err, datastore.ErrUserNotFound) {
			return false, nil
		}
		return false, err
	}

	return firstUser.UID == userID, nil
}

func firstOrgAdminInAnyOrg(ctx context.Context, userID string, orgMemberRepo datastore.OrganisationMemberRepository) (bool, error) {
	orgs, _, err := orgMemberRepo.LoadUserOrganisationsPaged(ctx, userID, datastore.Pageable{PerPage: 100})
	if err != nil {
		return false, err
	}

	for _, org := range orgs {
		member, err := orgMemberRepo.FetchOrganisationMemberByUserID(ctx, userID, org.UID)
		if err != nil {
			if errors.Is(err, datastore.ErrOrgMemberNotFound) {
				continue
			}
			return false, err
		}

		if member.Role.Type != auth.RoleOrganisationAdmin {
			continue
		}

		members, _, err := orgMemberRepo.LoadOrganisationMembersPaged(ctx, org.UID, "", datastore.Pageable{PerPage: 100})
		if err != nil {
			return false, err
		}

		var firstOrgAdmin *datastore.OrganisationMember
		for _, m := range members {
			if m.Role.Type == auth.RoleOrganisationAdmin {
				if firstOrgAdmin == nil || m.CreatedAt.Before(firstOrgAdmin.CreatedAt) {
					firstOrgAdmin = m
				}
			}
		}

		if firstOrgAdmin != nil && firstOrgAdmin.UserID == userID {
			return true, nil
		}
	}

	return false, nil
}
