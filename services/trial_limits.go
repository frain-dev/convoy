package services

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// ErrOrgUserLimit is returned when a cloud organisation is at its per-org team member cap
// (user_limit) for its current plan, e.g. a trial capping user_limit=1.
var ErrOrgUserLimit = errors.New("your organisation has reached its team member limit for its current plan, upgrade to add more members")

// ErrOrgOrganisationLimit is returned when the requesting user has reached the org_limit
// granted by their current plan, e.g. a trial capping org_limit=1.
var ErrOrgOrganisationLimit = errors.New("you have reached the organisation limit for your current plan, upgrade to create more organisations")

// firstPageCursor is the max-ULID cursor used to fetch the first page of a Next-direction
// listing; an empty cursor would compare o.id against the empty string and match nothing.
const firstPageCursor = "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF"

// OrgUserLimitDeps holds the repositories needed to enforce the per-org user_limit.
// InviteRepo is only required (and only used) when countPendingInvites is true.
type OrgUserLimitDeps struct {
	OrgMemberRepo datastore.OrganisationMemberRepository
	InviteRepo    datastore.OrganisationInviteRepository
	Logger        log.Logger
}

// CheckOrganisationUserLimit reports whether the organisation may add one more team member,
// enforcing the per-org user_limit against the org's own license_data (the single source of
// truth for cloud caps; no instance/platform fallback).
//
// countPendingInvites controls whether outstanding pending invites count toward the cap:
//   - true at invite-creation time: an org at its member cap with a pending invite is
//     effectively full, so counting pending invites (fail closed for trials) stops dangling
//     over-cap invites.
//   - false at accept-invite/member-create time: the invite being accepted is still pending
//     and must not count itself; only realised members matter there.
//
// Failure policy:
//   - fail OPEN (allowed=true) when no finite cap applies: empty/unreadable license_data or an
//     unlimited cap (this keeps self-hosted, which has no per-org license_data, unaffected).
//   - fail OPEN on a member/invite count lookup error: a transient DB fault must not block
//     legitimate member additions.
//   - fail CLOSED (allowed=false) only when a finite cap is resolved and the current head count
//     (members, plus pending invites when counted) has reached it.
func CheckOrganisationUserLimit(ctx context.Context, org *datastore.Organisation, countPendingInvites bool, deps OrgUserLimitDeps) (bool, error) {
	limit, applies := license.OrgEntitlementCap(org.UID, org.LicenseData, "user_limit")
	if !applies {
		return true, nil
	}

	members, err := deps.OrgMemberRepo.CountOrganisationMembers(ctx, org.UID)
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.Warn("user limit: member count lookup failed, allowing (fail open)", "error", err, "org_id", org.UID)
		}
		return true, nil
	}

	current := members
	if countPendingInvites && deps.InviteRepo != nil {
		pending, err := deps.InviteRepo.CountOrganisationInvites(ctx, org.UID, datastore.InviteStatusPending)
		if err != nil {
			if deps.Logger != nil {
				deps.Logger.Warn("user limit: pending invite count lookup failed, allowing (fail open)", "error", err, "org_id", org.UID)
			}
			return true, nil
		}
		current += pending
	}

	return current < limit, nil
}

// UserOrgLimitDeps holds the repository needed to enforce the per-org org_limit at
// org-creation time.
type UserOrgLimitDeps struct {
	OrgMemberRepo datastore.OrganisationMemberRepository
	Logger        log.Logger
}

// CheckUserOrgCreationAllowed reports whether the requesting user may create another
// organisation. Org creation is user-scoped, not org-scoped, so the applicable cap is taken
// from the plans of the organisations the user already belongs to: the user may not exceed
// the smallest finite org_limit among their existing orgs' license_data. A trialing user
// (org_limit=1, one org) is therefore blocked from creating a second org. This reads each
// org's own license_data (the single source of truth for cloud caps; no instance/platform
// fallback); the most restrictive applicable cap wins so the check is fail closed for a
// trialing org at cap.
//
// Failure policy:
//   - fail OPEN (allowed=true) when no finite per-org cap applies: none of the user's orgs
//     carry readable license_data with a finite org_limit (keeps self-hosted, whose orgs have
//     no per-org license_data, unaffected; the instance CheckOrgLimit still gates those).
//   - fail OPEN on a lookup error (loading the user's orgs or counting them): a transient DB
//     fault must not block org creation.
//   - fail CLOSED (allowed=false) only when a finite cap is resolved and the user's org count
//     has reached it.
func CheckUserOrgCreationAllowed(ctx context.Context, user *datastore.User, deps UserOrgLimitDeps) (bool, error) {
	orgs, _, err := deps.OrgMemberRepo.LoadUserOrganisationsPaged(ctx, user.UID, datastore.Pageable{
		PerPage:    100,
		Direction:  datastore.Next,
		NextCursor: firstPageCursor,
	})
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.Warn("org limit: user organisations lookup failed, allowing (fail open)", "error", err, "user_id", user.UID)
		}
		return true, nil
	}

	// Resolve the smallest finite org_limit across the user's existing orgs. -1 means
	// "no finite cap applies" => fail open.
	limit := int64(-1)
	for i := range orgs {
		if orgCap, applies := license.OrgEntitlementCap(orgs[i].UID, orgs[i].LicenseData, "org_limit"); applies {
			if limit == -1 || orgCap < limit {
				limit = orgCap
			}
		}
	}
	if limit == -1 {
		return true, nil
	}

	count, err := deps.OrgMemberRepo.CountUserOrganisations(ctx, user.UID, "")
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.Warn("org limit: user organisation count lookup failed, allowing (fail open)", "error", err, "user_id", user.UID)
		}
		return true, nil
	}

	return count < limit, nil
}
