package auth

import (
	"fmt"
	"strings"
)

type Role string

const (
	RoleSuperUser = Role("super_user")
	RoleUIAdmin   = Role("ui_admin")
	RoleAdmin     = Role("admin")
	RoleAPI       = Role("api")
)

func (r *Role) UnmarshalJSON(b []byte) error {
	str := strings.Trim(string(b), `"`)
	*r = Role(str)

	if !r.IsValid() {
		return fmt.Errorf("invalid role %s", r.String())
	}
	return nil
}

func (r Role) IsValid() bool {
	switch r {
	case RoleSuperUser, RoleUIAdmin, RoleAdmin, RoleAPI:
		return true
	default:
		return false
	}
}

func (r Role) String() string {
	return string(r)
}
