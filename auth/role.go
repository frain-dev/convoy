package auth

import (
	"fmt"
)

// Role represents the permission a user is given, if the Type is RoleOrganisationAdmin,
// Then the user will have access to everything regardless of the value of Project.
type Role struct {
	Type     RoleType `json:"type" db:"type"`
	Project  string   `json:"project" db:"project"`
	Endpoint string   `json:"endpoint,omitempty" db:"endpoint"`
}

type RoleType string

const (
	RoleRoot              = RoleType("root")
	RoleInstanceAdmin     = RoleType("instance_admin")
	RoleOrganisationAdmin = RoleType("organisation_admin")
	RoleAdmin             = RoleType("admin")
	RoleMember            = RoleType("member")
	RoleAPI               = RoleType("api")
)

func (r RoleType) IsValid() bool {
	switch r {
	case RoleRoot, RoleInstanceAdmin, RoleOrganisationAdmin, RoleAdmin, RoleMember, RoleAPI:
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

	return nil
}
