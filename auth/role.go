package auth

import (
	"fmt"
)

// Role represents the permission a user is given, if the Type is RoleSuperUser,
// Then the user will have access to everything regardless of the value of Group.
type Role struct {
	Type         RoleType `json:"type"`
	Organisation string   `json:"orgnisation"`
	Group        string   `json:"group"`
	App          string   `json:"app,omitempty"`
}

type RoleType string

const (
	RoleSuperUser = RoleType("super_user")
	RoleAdmin     = RoleType("admin")
	RoleAPI       = RoleType("api")
)

func (r RoleType) IsValid() bool {
	switch r {
	case RoleSuperUser, RoleAdmin, RoleAPI:
		return true
	default:
		return false
	}
}

func (r *Role) HasGroup(groupID string) bool {
	return r.Group == groupID
}

func (r *Role) HasApp(appID string) bool {
	return r.App == appID
}

func (r RoleType) String() string {
	return string(r)
}

func (r RoleType) Is(rt RoleType) bool {
	return r == rt
}

func (r *Role) Validate(credType string) error {
	if !r.Type.IsValid() {
		return fmt.Errorf("invalid role type: %s", r.Type.String())
	}

	// group will never be checked for superuser
	if r.Group == "" && !r.Type.Is(RoleSuperUser) {
		return fmt.Errorf("please specify group for %s", credType)
	}

	return nil
}
