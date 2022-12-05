package auth

import (
	"fmt"
)

// Role represents the permission a user is given, if the Type is RoleSuperUser,
// Then the user will have access to everything regardless of the value of Project.
type Role struct {
	Type     RoleType `json:"type"`
	Project  string   `json:"project"`
	Endpoint string   `json:"endpoint,omitempty"`
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

func (r *Role) HasProject(projectID string) bool {
	return r.Project == projectID
}

func (r *Role) HasEndpoint(endpointID string) bool {
	return r.Endpoint == endpointID
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

	// projects will never be checked for superuser
	if r.Project == "" && !r.Type.Is(RoleSuperUser) {
		return fmt.Errorf("please specify project for %s", credType)
	}

	return nil
}
