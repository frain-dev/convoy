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
	PermissionManageAll          Permission = "manage.all"
	PermissionManage             Permission = "manage"
	PermissionAdd                Permission = "add"
	PermissionView               Permission = "view"
	PermissionOrganisationManage Permission = "organisation.manage"
	PermissionOrganisationAdd    Permission = "organisation.add"
	PermissionProjectManage      Permission = "project.manage"
	PermissionProjectView        Permission = "project.view"
)
