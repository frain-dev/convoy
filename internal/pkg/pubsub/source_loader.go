package pubsub

import (
	"context"

	"github.com/frain-dev/convoy/internal/pkg/memorystore"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

const (
	perPage = 50
)

type SourceLoader struct {
	endpointRepo datastore.EndpointRepository
	sourceRepo   datastore.SourceRepository
	projectRepo  datastore.ProjectRepository

	log log.StdLogger
}

func NewSourceLoader(endpointRepo datastore.EndpointRepository, sourceRepo datastore.SourceRepository, projectRepo datastore.ProjectRepository, log log.StdLogger) *SourceLoader {
	return &SourceLoader{
		endpointRepo: endpointRepo,
		sourceRepo:   sourceRepo,
		projectRepo:  projectRepo,
		log:          log,
	}
}

// TODO(subomi): Refactor source loader to not know about table
// instead it should return changes through a channel.
func (s *SourceLoader) SyncChanges(ctx context.Context, table *memorystore.Table) error {
	mSourceKeys := table.GetKeys()

	sources, err := s.fetchProjectSources(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to fetch sources")
		return err
	}

	var dSourceKeys []string
	for _, source := range sources {
		dSourceKeys = append(dSourceKeys, generateSourceKey(&source))
	}

	// find new and updated rows
	newRows := util.Difference(dSourceKeys, mSourceKeys)
	if len(newRows) != 0 {
		for _, idx := range newRows {
			for _, source := range sources {
				if generateSourceKey(&source) == idx {
					_ = table.Add(idx, source)
				}
			}
		}
	}

	// find deleted rows
	deletedRows := util.Difference(mSourceKeys, dSourceKeys)
	if len(deletedRows) != 0 {
		for _, idx := range deletedRows {
			table.Delete(idx)
		}
	}

	return nil
}

func (s *SourceLoader) fetchSources(ctx context.Context, sources []datastore.Source, projectIDs []string, cursor string) ([]datastore.Source, error) {
	pageable := datastore.Pageable{
		NextCursor: cursor,
		Direction:  datastore.Next,
		PerPage:    perPage,
	}

	newSources, pagination, err := s.sourceRepo.LoadPubSubSourcesByProjectIDs(ctx, projectIDs, pageable)
	if err != nil {
		return nil, err
	}

	if len(newSources) == 0 && !pagination.HasNextPage {
		return sources, nil
	}

	if pagination.HasNextPage {
		cursor = pagination.NextPageCursor
		sources = append(sources, newSources...)
		return s.fetchSources(ctx, sources, projectIDs, cursor)
	}

	sources = append(sources, newSources...)
	return sources, nil
}

func (s *SourceLoader) fetchProjectSources(ctx context.Context) ([]datastore.Source, error) {
	projects, err := s.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(projects))
	for i := range projects {
		ids[i] = projects[i].UID
	}

	var sources []datastore.Source
	sources, err = s.fetchSources(ctx, sources, ids, "")
	if err != nil {
		s.log.WithError(err).Error("failed to load sources")
		return nil, err
	}

	return sources, nil
}
