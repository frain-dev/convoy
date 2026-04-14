package api

import (
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/projects"
)

// ensureAPIRepositories wires organisation and project repositories when the caller
// omitted them but provided DB and Logger (e.g. cmd/server or dataplane startup).
func ensureAPIRepositories(a *types.APIOptions) {
	if a == nil || a.DB == nil || a.Logger == nil {
		return
	}
	if a.OrgRepo == nil {
		a.OrgRepo = organisations.New(a.Logger, a.DB)
	}
	if a.OrgMemberRepo == nil {
		a.OrgMemberRepo = organisation_members.New(a.Logger, a.DB)
	}
	if a.ProjectRepo == nil {
		a.ProjectRepo = projects.New(a.Logger, a.DB)
	}
}
