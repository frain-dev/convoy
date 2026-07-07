package api

import (
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/datastore/cached"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/projects"
)

// ensureAPIRepositories wires organisation and project repositories when the caller
// omitted them but provided DB and Logger (e.g. cmd/server or dataplane startup).
// The project repository is wrapped in the read-through cache whenever a cache is
// available so every write path invalidates "projects:<id>" instead of serving
// stale config until the TTL expires.
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
		projectRepo := projects.New(a.Logger, a.DB)
		if a.Cache != nil {
			a.ProjectRepo = cached.NewCachedProjectRepository(projectRepo, a.Cache, cached.DefaultProjectTTL, a.Logger)
		} else {
			a.ProjectRepo = projectRepo
		}
	}
}
