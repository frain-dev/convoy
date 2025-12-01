package policies

import (
	"errors"

	"github.com/frain-dev/convoy/api/types"
)

const AuthUserCtx types.ContextKey = "authUser"

// ErrNotAllowed is returned when the request is not permitted.
var ErrNotAllowed = errors.New("unauthorized to process request")

// Permission is a type for all permission keys used in the system.
type Permission string

const (
	PermissionOrganisationBase Permission = "organisation"
	PermissionProjectBase      Permission = "project"
	PermissionBillingBase      Permission = "billing"
	PermissionManage           Permission = "manage"
	PermissionAdd              Permission = "add"
	PermissionView             Permission = "view"
	PermissionAll              Permission = "all"

	PermissionManageAll = PermissionManage + "." + PermissionAll

	PermissionOrganisationManage    = PermissionOrganisationBase + "." + PermissionManage
	PermissionOrganisationAdd       = PermissionOrganisationBase + "." + PermissionAdd
	PermissionOrganisationManageAll = PermissionOrganisationBase + "." + PermissionManageAll
	PermissionProjectManage         = PermissionProjectBase + "." + PermissionManage
	PermissionProjectView           = PermissionProjectBase + "." + PermissionView
	PermissionBillingManage         = PermissionBillingBase + "." + PermissionManage
)
