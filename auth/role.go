package auth

// Role represents the permission a user is given, if the Type is RoleSuperUser,
// Then the user will have access to everything regardless of the value of Groups.
type Role struct {
	Type   RoleType `json:"type"`
	Groups []string `json:"groups"`
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
