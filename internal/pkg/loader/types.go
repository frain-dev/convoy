package loader

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
)

// Error definitions
var (
	ErrInvalidBatchSize = errors.New("batch size must be greater than 0")
)

// SubscriptionTableManager abstracts table operations for better testability
type SubscriptionTableManager interface {
	AddSubscription(sub datastore.Subscription, table *memorystore.Table)
	RemoveSubscription(sub datastore.Subscription, table *memorystore.Table)
	RemoveSubscriptionFromAllEventTypes(sub datastore.Subscription, table *memorystore.Table)
}

// SubscriptionFetcher abstracts data access operations
type SubscriptionFetcher interface {
	FetchAllSubscriptions(ctx context.Context) ([]datastore.Subscription, error)
	FetchUpdatedSubscriptions(ctx context.Context) ([]datastore.Subscription, error)
	FetchNewSubscriptions(ctx context.Context) ([]datastore.Subscription, error)
	FetchDeletedSubscriptions(ctx context.Context) ([]datastore.Subscription, error)
}

// ProjectIDExtractor handles project ID extraction logic
type ProjectIDExtractor struct {
	projectRepo datastore.ProjectRepository
}

// NewProjectIDExtractor creates a new project ID extractor
func NewProjectIDExtractor(projectRepo datastore.ProjectRepository) *ProjectIDExtractor {
	return &ProjectIDExtractor{
		projectRepo: projectRepo,
	}
}

// ExtractProjectIDs fetches all project IDs from the repository
func (p *ProjectIDExtractor) ExtractProjectIDs(ctx context.Context) ([]string, error) {
	projects, err := p.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		return []string{}, nil
	}

	ids := make([]string, len(projects))
	for i := range projects {
		ids[i] = projects[i].UID
	}

	return ids, nil
}

// SubscriptionCollection manages a collection of subscription updates
type SubscriptionCollection struct {
	updates []datastore.SubscriptionUpdate
}

// NewSubscriptionCollection creates a new subscription collection
func NewSubscriptionCollection() *SubscriptionCollection {
	return &SubscriptionCollection{
		updates: make([]datastore.SubscriptionUpdate, 0),
	}
}

// AddOrUpdate adds a subscription update or updates existing one
func (sc *SubscriptionCollection) AddOrUpdate(sub datastore.Subscription) {
	for i, update := range sc.updates {
		if update.UID == sub.UID {
			sc.updates[i].UpdatedAt = sub.UpdatedAt
			return
		}
	}

	sc.updates = append(sc.updates, datastore.SubscriptionUpdate{
		UID:       sub.UID,
		UpdatedAt: sub.UpdatedAt,
	})
}

// Remove removes a subscription update by UID
func (sc *SubscriptionCollection) Remove(uid string) {
	for i, update := range sc.updates {
		if update.UID == uid {
			sc.updates = append(sc.updates[:i], sc.updates[i+1:]...)
			return
		}
	}
}

// GetAll returns all subscription updates
func (sc *SubscriptionCollection) GetAll() []datastore.SubscriptionUpdate {
	return sc.updates
}
