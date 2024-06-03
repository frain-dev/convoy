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
	DefaultBatchSize = 10_000
)

type SubscriptionLoader struct {
	subRepo     datastore.SubscriptionRepository
	projectRepo datastore.ProjectRepository

	loaded        bool
	batchSize     int64
	lastUpdatedAt time.Time
	lastDelete    time.Time
	log           log.StdLogger
}

func NewSubscriptionLoader(subRepo datastore.SubscriptionRepository, projectRepo datastore.ProjectRepository, log log.StdLogger, batchSize int64) *SubscriptionLoader {
	if batchSize == 0 {
		batchSize = DefaultBatchSize
	}

	return &SubscriptionLoader{
		log:         log,
		batchSize:   batchSize,
		subRepo:     subRepo,
		projectRepo: projectRepo,
	}
}

func (s *SubscriptionLoader) SyncChanges(ctx context.Context, table *memorystore.Table) error {
	if !s.loaded {
		startTime := time.Now()
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

			s.addSubscriptionToTable(sub, table)
		}

		s.loaded = true
		s.log.Info("syncing subscriptions completed in ", time.Since(startTime))
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

			s.addSubscriptionToTable(sub, table)
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

		s.deleteSubscriptionToTable(sub, table)
	}

	return nil
}

func (s *SubscriptionLoader) addSubscriptionToTable(sub datastore.Subscription, table *memorystore.Table) {
	if sub.FilterConfig == nil {
		return
	}

	eventTypes := sub.FilterConfig.EventTypes
	if len(eventTypes) == 0 {
		return
	}

	for _, ev := range eventTypes {
		key := memorystore.NewKey(sub.ProjectID, ev)
		row := table.Get(key)

		var values []datastore.Subscription
		var ok bool

		switch {
		case row != nil:
			values, ok = row.Value().([]datastore.Subscription)
			if !ok {
				log.Errorf("malformed data in subscriptions memory store with event type: %s", ev)
				continue
			}
		default:
			values = make([]datastore.Subscription, 0)
		}

		if s.loaded {
			for id, v := range values {
				if v.UID == sub.UID {
					b := values[:id]
					a := values[id+1:]
					values = append(b, a...)
					break
				}
			}
		}

		values = append(values, sub)
		table.Upsert(key, values)
	}
}

func (s *SubscriptionLoader) deleteSubscriptionToTable(sub datastore.Subscription, table *memorystore.Table) {
	if sub.FilterConfig == nil {
		return
	}

	eventTypes := sub.FilterConfig.EventTypes
	if len(eventTypes) == 0 {
		return
	}

	for _, ev := range eventTypes {
		key := memorystore.NewKey(sub.ProjectID, ev)
		row := table.Get(key)

		if row == nil {
			continue
		}

		var values []datastore.Subscription
		if row.Value() != nil {
			var ok bool
			values, ok = row.Value().([]datastore.Subscription)
			if !ok {
				log.Errorf("malformed data in subscriptions memory store with event type: %s", ev)
				continue
			}
		}

		for id, v := range values {
			if v.UID == sub.UID {
				b := values[:id]
				a := values[id+1:]
				values = append(b, a...)
				break
			}
		}

		if len(values) == 0 {
			table.Delete(key)
			return
		}

		table.Upsert(key, values)
	}
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
			subscriptions, err := s.subRepo.FetchUpdatedSubscriptions(ctx, projectID, s.lastUpdatedAt, s.batchSize)
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
			subscriptions, err := s.subRepo.FetchDeletedSubscriptions(ctx, projectID, s.lastUpdatedAt, s.batchSize)
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
