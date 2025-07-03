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

func roleRank(rt RoleType) int {
	switch rt {
	case RoleInstanceAdmin:
		return 5
	case RoleOrganisationAdmin:
		return 4
	case RoleBillingAdmin:
		return 3
	case RoleProjectAdmin:
		return 2
	case RoleProjectViewer:
		return 1
	case RoleAPI:
		return 0 // deprecated or lowest
	default:
		return -1 // unknown role
	}
}

func (r RoleType) IsAtLeast(rt RoleType) bool {
	return roleRank(r) >= roleRank(rt)
}

func (r *Role) Validate(credType string) error {
	if !r.Type.IsValid() {
		return fmt.Errorf("invalid role type: %s", r.Type.String())
	}

	return nil
}
