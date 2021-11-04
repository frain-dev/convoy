package auth

import (
	"fmt"
	"strings"
)

// Role represents the permission a user is given, if the Type is RoleSuperUser,
// Then the user will have access to everything regardless of the value of Group.
type Role struct {
	Type  RoleType `json:"type"`
	Group string   `json:"group"`
}

type RoleType string

const (
	RoleSuperUser = RoleType("super_user")
	RoleUIAdmin   = RoleType("ui_admin")
	RoleAdmin     = RoleType("admin")
	RoleAPI       = RoleType("api")
)

func (r *RoleType) UnmarshalJSON(b []byte) error {
	str := strings.Trim(string(b), `"`)
	*r = RoleType(str)

	if !r.IsValid() {
		return fmt.Errorf("invalid role %s", r.String())
	}
	return nil
}

func (r RoleType) IsValid() bool {
	switch r {
	case RoleSuperUser, RoleUIAdmin, RoleAdmin, RoleAPI:
		return true
	default:
		return false
	}
}

func (r RoleType) String() string {
	return string(r)
}
