package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/cached"
	"github.com/frain-dev/convoy/pkg/cachedrepo"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

// SoftDeleteOrganisationDeps holds repositories used when soft-deleting an
// organisation with the minimal security cascade (keys, sources, caches).
type SoftDeleteOrganisationDeps struct {
	ProjectRepo datastore.ProjectRepository
	DB          database.Database
	Cache       cache.Cache
	Logger      log.Logger
}

// SoftDeleteOrganisationWithCascade soft-deletes an organisation after revoking
// project-scoped API keys and sources and invalidating org/project caches.
// Full product cascade (members, invites, deliveries retention) is out of scope.
// Failure policy: fail closed — key/source revoke and org soft-delete run in one
// transaction; cache invalidation runs only after commit.
func SoftDeleteOrganisationWithCascade(ctx context.Context, deps SoftDeleteOrganisationDeps, orgID string) error {
	projects, err := deps.ProjectRepo.LoadProjects(ctx, &datastore.ProjectFilter{OrgID: orgID})
	if err != nil {
		return fmt.Errorf("list organisation projects: %w", err)
	}

	pool := deps.DB.GetConn()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin organisation delete transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	projectIDs := make([]string, 0, len(projects))
	maskIDs := make([]string, 0)
	for _, project := range projects {
		if project == nil {
			continue
		}
		projectIDs = append(projectIDs, project.UID)

		// Collect mask IDs before revoke so apikeys_by_mask:* cache entries can
		// be invalidated after commit (native auth caches keys for ~5m).
		rows, qerr := tx.Query(ctx, `
			SELECT mask_id
			FROM convoy.api_keys
			WHERE role_project = $1 AND deleted_at IS NULL AND mask_id IS NOT NULL AND mask_id <> ''
		`, project.UID)
		if qerr != nil {
			return fmt.Errorf("list api key masks for project %s: %w", project.UID, qerr)
		}
		for rows.Next() {
			var maskID string
			if err = rows.Scan(&maskID); err != nil {
				rows.Close()
				return fmt.Errorf("scan api key mask for project %s: %w", project.UID, err)
			}
			maskIDs = append(maskIDs, maskID)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return fmt.Errorf("iterate api key masks for project %s: %w", project.UID, err)
		}

		if _, err = tx.Exec(ctx, `
			UPDATE convoy.api_keys
			SET deleted_at = NOW()
			WHERE role_project = $1 AND deleted_at IS NULL
		`, project.UID); err != nil {
			return fmt.Errorf("revoke api keys for project %s: %w", project.UID, err)
		}
		if _, err = tx.Exec(ctx, `
			UPDATE convoy.sources
			SET deleted_at = NOW()
			WHERE project_id = $1 AND deleted_at IS NULL
		`, project.UID); err != nil {
			return fmt.Errorf("soft-delete sources for project %s: %w", project.UID, err)
		}
	}

	result, err := tx.Exec(ctx, `
		UPDATE convoy.organisations
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, orgID)
	if err != nil {
		return fmt.Errorf("soft-delete organisation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return util.NewServiceError(http.StatusNotFound, datastore.ErrOrgNotFound)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit organisation delete transaction: %w", err)
	}

	if deps.Cache != nil && deps.Logger != nil {
		for _, projectID := range projectIDs {
			cachedrepo.Invalidate(ctx, deps.Cache, deps.Logger, "projects:"+projectID)
		}
		for _, maskID := range maskIDs {
			cachedrepo.Invalidate(ctx, deps.Cache, deps.Logger, "apikeys_by_mask:"+maskID)
		}
		cachedrepo.Invalidate(ctx, deps.Cache, deps.Logger, cached.OrganisationCacheKey(orgID))
	}

	return nil
}
