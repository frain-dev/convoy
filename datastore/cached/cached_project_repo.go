package cached

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const projectKeyPrefix = "projects"

type CachedProjectRepository struct {
	inner  datastore.ProjectRepository
	cache  cache.Cache
	ttl    time.Duration
	logger log.Logger
}

func NewCachedProjectRepository(inner datastore.ProjectRepository, c cache.Cache, ttl time.Duration, logger log.Logger) *CachedProjectRepository {
	return &CachedProjectRepository{
		inner:  inner,
		cache:  c,
		ttl:    ttl,
		logger: logger,
	}
}

func projectCacheKey(projectID string) string {
	return fmt.Sprintf("%s:%s", projectKeyPrefix, projectID)
}

func (c *CachedProjectRepository) FetchProjectByID(ctx context.Context, id string) (*datastore.Project, error) {
	key := projectCacheKey(id)

	var project datastore.Project
	err := c.cache.Get(ctx, key, &project)
	if err != nil {
		c.logger.Error("cache get error for project", "key", key, "error", err)
	}

	if project.UID != "" {
		return &project, nil
	}

	// Cache miss -- fetch from DB
	p, err := c.inner.FetchProjectByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if setErr := c.cache.Set(ctx, key, p, c.ttl); setErr != nil {
		c.logger.Error("cache set error for project", "key", key, "error", setErr)
	}

	return p, nil
}

func (c *CachedProjectRepository) UpdateProject(ctx context.Context, project *datastore.Project) error {
	err := c.inner.UpdateProject(ctx, project)
	if err != nil {
		return err
	}

	c.invalidateProject(ctx, project.UID)
	return nil
}

func (c *CachedProjectRepository) DeleteProject(ctx context.Context, uid string) error {
	err := c.inner.DeleteProject(ctx, uid)
	if err != nil {
		return err
	}

	c.invalidateProject(ctx, uid)
	return nil
}

func (c *CachedProjectRepository) invalidateProject(ctx context.Context, projectID string) {
	key := projectCacheKey(projectID)
	if err := c.cache.Delete(ctx, key); err != nil {
		c.logger.Error("cache delete error for project", "key", key, "error", err)
	}
}

// Passthrough methods

func (c *CachedProjectRepository) LoadProjects(ctx context.Context, filter *datastore.ProjectFilter) ([]*datastore.Project, error) {
	return c.inner.LoadProjects(ctx, filter)
}

func (c *CachedProjectRepository) CreateProject(ctx context.Context, project *datastore.Project) error {
	return c.inner.CreateProject(ctx, project)
}

func (c *CachedProjectRepository) CountProjects(ctx context.Context) (int64, error) {
	return c.inner.CountProjects(ctx)
}

func (c *CachedProjectRepository) GetProjectsWithEventsInTheInterval(ctx context.Context, interval int) ([]datastore.ProjectEvents, error) {
	return c.inner.GetProjectsWithEventsInTheInterval(ctx, interval)
}

func (c *CachedProjectRepository) FillProjectsStatistics(ctx context.Context, project *datastore.Project) error {
	return c.inner.FillProjectsStatistics(ctx, project)
}
