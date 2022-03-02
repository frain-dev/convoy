package auth

import "fmt"

// Role represents the permission a user is given, if the Type is RoleSuperUser,
// Then the user will have access to everything regardless of the value of Groups.
type Role struct {
	Type   RoleType `json:"type"`
	Groups []string `json:"groups"`
	Apps   []string `json:"apps,omitempty"`
}

type RoleType string

const (
	RoleSuperUser = RoleType("super_user")
	RoleUIAdmin   = RoleType("ui_admin")
	RoleAdmin     = RoleType("admin")
	RoleAPI       = RoleType("api")
)

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

func (r RoleType) Is(rt RoleType) bool {
	return r == rt
}

func (r *Role) Validate(credType string) error {
	if !r.Type.IsValid() {
		return fmt.Errorf("invalid role type: %s", r.Type.String())
	}

	// groups will never be checked for superusers
	if len(r.Groups) == 0 && !r.Type.Is(RoleSuperUser) {
		return fmt.Errorf("please specify groups for %s", credType)
	}

	for _, group := range r.Groups {
		if group == "" {
			return fmt.Errorf("empty group name not allowed for %s", credType)
		}
	}
	return nil
}
