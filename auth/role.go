package auth

import (
	"fmt"
)

// Role represents the permission a user is given, if the Type is RoleInstanceAdmin,
// Then the user will have access to everything regardless of the value of Project.
type Role struct {
	Type     RoleType `json:"type" db:"type"`
	Project  string   `json:"project" db:"project"`
	Endpoint string   `json:"endpoint,omitempty" db:"endpoint"`
}

type RoleType string

const (
	RoleInstanceAdmin     = RoleType("instance_admin")     // Instance level - can manage all orgs and projects
	RoleOrganisationAdmin = RoleType("organisation_admin") // Organisation level - can manage org and all projects
	RoleBillingAdmin      = RoleType("billing_admin")      // Organisation level - can manage billing only TODO:
	RoleProjectAdmin      = RoleType("project_admin")      // Project level - can manage project settings and users
	RoleProjectViewer     = RoleType("project_viewer")     // Project level - can view project data only
	// RoleAPI Deprecated
	RoleAPI = RoleType("api")
)

func (r RoleType) IsValid() bool {
	switch r {
	case RoleInstanceAdmin, RoleOrganisationAdmin, RoleBillingAdmin, RoleProjectAdmin, RoleProjectViewer, RoleAPI:
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
