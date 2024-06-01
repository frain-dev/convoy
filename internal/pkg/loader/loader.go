package loader

import (
	"context"
	"sync"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/pkg/log"
)

const (
	perPage = 10_000
)

type SubscriptionLoader struct {
	subRepo     datastore.SubscriptionRepository
	projectRepo datastore.ProjectRepository

	loaded        bool
	batchSize     int
	lastUpdatedAt time.Time
	lastDelete    time.Time
	log           log.StdLogger
}

func NewSubscriptionLoader(subRepo datastore.SubscriptionRepository, projectRepo datastore.ProjectRepository, log log.StdLogger) *SubscriptionLoader {
	return &SubscriptionLoader{
		log:         log,
		batchSize:   perPage,
		subRepo:     subRepo,
		projectRepo: projectRepo,
	}
}

func (s *SubscriptionLoader) SyncChanges(ctx context.Context, table *memorystore.Table) error {
	var key memorystore.Key

	if !s.loaded {
		// fetch subscriptions.
		subscriptions, err := s.fetchAllSubscriptions(ctx)
		if err != nil {
			s.log.WithError(err).Error("failed to fetch subscriptions")
			return err
		}

		for _, sub := range subscriptions {
			if sub.UpdatedAt.After(s.lastUpdatedAt) {
				s.lastUpdatedAt = sub.UpdatedAt
			}

			key = memorystore.NewKey(sub.ProjectID, sub.UID)
			table.Add(key, sub)
		}

		s.loaded = true
		return nil
	}

	// fetch subscriptions.
	updatedSubs, err := s.fetchUpdatedSubscriptions(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to fetch subscriptions")
		return err
	}

	if len(updatedSubs) != 0 {
		for _, sub := range updatedSubs {
			if sub.UpdatedAt.After(s.lastUpdatedAt) {
				s.lastUpdatedAt = sub.UpdatedAt
			}

			key = memorystore.NewKey(sub.ProjectID, sub.UID)
			_ = table.Upsert(key, sub)
		}
	}

	// fetch subscriptions.
	deletedSubs, err := s.fetchDeletedSubscriptions(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to fetch subscriptions")
		return err
	}

	for _, sub := range deletedSubs {
		if sub.DeletedAt.Time.After(s.lastDelete) {
			s.lastDelete = sub.DeletedAt.Time
		}

		key = memorystore.NewKey(sub.ProjectID, sub.UID)
		table.Delete(key)
	}

	return nil
}

func (s *SubscriptionLoader) fetchAllSubscriptions(ctx context.Context) ([]datastore.Subscription, error) {
	projects, err := s.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	resultChan := make(chan []datastore.Subscription, len(projects))

	for i := range projects {
		wg.Add(1)

		go func(projectID string) {
			defer wg.Done()

			var subscriptions []datastore.Subscription
			subscriptions, err := s.subRepo.LoadAllSubscriptionConfig(ctx, projectID, s.batchSize)
			if err != nil {
				s.log.WithError(err).Errorf("failed to load subscriptions of project %s", projectID)
				return
			}

			resultChan <- subscriptions
		}(projects[i].UID)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allSubscriptions []datastore.Subscription
	for projectSubs := range resultChan {
		allSubscriptions = append(allSubscriptions, projectSubs...)
	}

	return allSubscriptions, nil
}

func (s *SubscriptionLoader) fetchUpdatedSubscriptions(ctx context.Context) ([]datastore.Subscription, error) {
	projects, err := s.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	resultChan := make(chan []datastore.Subscription, len(projects))

	for i := range projects {
		wg.Add(1)

		go func(projectID string) {
			defer wg.Done()

			var subscriptions []datastore.Subscription
			subscriptions, err := s.subRepo.FetchUpdatedSubscriptions(ctx, projectID, s.lastUpdatedAt, 10_000)
			if err != nil {
				s.log.WithError(err).Errorf("failed to load subscriptions of project %s", projectID)
				return
			}

			resultChan <- subscriptions
		}(projects[i].UID)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allSubscriptions []datastore.Subscription
	for projectSubs := range resultChan {
		allSubscriptions = append(allSubscriptions, projectSubs...)
	}

	return allSubscriptions, nil
}

func (s *SubscriptionLoader) fetchDeletedSubscriptions(ctx context.Context) ([]datastore.Subscription, error) {
	projects, err := s.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	resultChan := make(chan []datastore.Subscription, len(projects))

	for i := range projects {
		wg.Add(1)

		go func(projectID string) {
			defer wg.Done()

			var subscriptions []datastore.Subscription
			subscriptions, err := s.subRepo.FetchDeletedSubscriptions(ctx, projectID, s.lastUpdatedAt, perPage)
			if err != nil {
				s.log.WithError(err).Errorf("failed to load subscriptions of project %s", projectID)
				return
			}

			resultChan <- subscriptions
		}(projects[i].UID)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allSubscriptions []datastore.Subscription
	for projectSubs := range resultChan {
		allSubscriptions = append(allSubscriptions, projectSubs...)
	}

	return allSubscriptions, nil
}
