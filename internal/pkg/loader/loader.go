package loader

import (
	"context"
	"fmt"
	"strings"
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

	loaded              bool
	batchSize           int64
	subscriptionUpdates []datastore.SubscriptionUpdate
	log                 log.StdLogger
}

func NewSubscriptionLoader(subRepo datastore.SubscriptionRepository, projectRepo datastore.ProjectRepository, log log.StdLogger, batchSize int64) *SubscriptionLoader {
	if batchSize == 0 {
		batchSize = DefaultBatchSize
	}

	return &SubscriptionLoader{
		log:                 log,
		subscriptionUpdates: make([]datastore.SubscriptionUpdate, 0),
		batchSize:           batchSize,
		subRepo:             subRepo,
		projectRepo:         projectRepo,
	}
}

func (s *SubscriptionLoader) SyncChanges(ctx context.Context, table *memorystore.Table) error {
	if !s.loaded {
		startTime := time.Now()
		// fetch subscriptions.
		subscriptions, err := s.fetchAllSubscriptions(ctx)
		if err != nil {
			s.log.WithError(err).Error("failed to fetch all subscriptions")
			return err
		}

		for _, sub := range subscriptions {
			s.subscriptionUpdates = append(s.subscriptionUpdates, datastore.SubscriptionUpdate{
				UID:       sub.UID,
				UpdatedAt: sub.UpdatedAt,
			})
			s.addSubscriptionToTable(sub, table)
		}

		s.loaded = true
		s.log.Infof("syncing subscriptions completed in %fs", time.Since(startTime).Seconds())
		return nil
	}

	for _, key := range table.GetKeys() {
		value := table.Get(key)
		subs, ok := value.Value().([]datastore.Subscription)
		if !ok {
			continue
		}
		subIDs := make([]string, len(subs))
		for i, sub := range subs {
			subIDs[i] = sub.UID
		}
		fmt.Printf("Key: %s, Subscription IDs: %s\n", key, strings.Join(subIDs, ", "))
	}

	// fetch subscriptions.
	updatedSubs, err := s.fetchUpdatedSubscriptions(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to fetch updated subscriptions")
		return err
	}

	for _, sub := range updatedSubs {
		for i, update := range s.subscriptionUpdates {
			if update.UID == sub.UID {
				s.subscriptionUpdates[i].UpdatedAt = sub.UpdatedAt
				break
			}
		}
		s.addSubscriptionToTable(sub, table)
	}

	// fetch subscriptions.
	deletedSubs, err := s.fetchDeletedSubscriptions(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to fetch deleted subscriptions")
		return err
	}

	for _, sub := range deletedSubs {
		s.deleteSubscriptionFromTable(sub, table)
		for i, update := range s.subscriptionUpdates {
			if update.UID == sub.UID {
				s.subscriptionUpdates = append(s.subscriptionUpdates[:i], s.subscriptionUpdates[i+1:]...)
				break
			}
		}
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

	// If this is an update (not initial load), first remove the subscription from all event types
	// for this project to handle cases where event types have changed
	if s.loaded {
		s.removeSubscriptionFromAllEventTypes(sub, table)
	}

	// Now add the subscription to its current event types
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

		// Remove the subscription if it already exists (shouldn't happen after the above removal, but just in case)
		for id, v := range values {
			if v.UID == sub.UID {
				b := values[:id]
				a := values[id+1:]
				values = append(b, a...)
				break
			}
		}

		values = append(values, sub)
		table.Upsert(key, values)
	}
}

// removeSubscriptionFromAllEventTypes removes a subscription from all event types in the table for a given project
func (s *SubscriptionLoader) removeSubscriptionFromAllEventTypes(sub datastore.Subscription, table *memorystore.Table) {
	// Get all keys in the table for this project
	keys := table.GetKeys()

	for _, key := range keys {
		// Only process keys for this project
		if !key.HasPrefix(sub.ProjectID) {
			continue
		}

		row := table.Get(key)
		if row == nil {
			continue
		}

		var values []datastore.Subscription
		var ok bool
		values, ok = row.Value().([]datastore.Subscription)
		if !ok {
			log.Errorf("malformed data in subscriptions memory store with key: %s", key)
			continue
		}

		// Remove the subscription if it exists in this event type
		found := false
		for id, v := range values {
			if v.UID == sub.UID {
				b := values[:id]
				a := values[id+1:]
				values = append(b, a...)
				found = true
				break
			}
		}

		// Update or delete the key based on whether any subscriptions remain
		if found {
			if len(values) == 0 {
				table.Delete(key)
			} else {
				table.Upsert(key, values)
			}
		}
	}
}

func (s *SubscriptionLoader) deleteSubscriptionFromTable(sub datastore.Subscription, table *memorystore.Table) {
	// When deleting a subscription, we need to remove it from ALL event types
	// in the table for this project, not just the ones specified in FilterConfig.EventTypes
	// This is because the subscription might have been associated with other event types
	// that are no longer reflected in the current FilterConfig

	// Get all keys in the table for this project
	keys := table.GetKeys()

	for _, key := range keys {
		// Only process keys for this project
		if !key.HasPrefix(sub.ProjectID) {
			continue
		}

		row := table.Get(key)
		if row == nil {
			continue
		}

		var values []datastore.Subscription
		var ok bool
		values, ok = row.Value().([]datastore.Subscription)
		if !ok {
			log.Errorf("malformed data in subscriptions memory store with key: %s", key)
			continue
		}

		// Remove the subscription if it exists in this event type
		found := false
		for id, v := range values {
			if v.UID == sub.UID {
				b := values[:id]
				a := values[id+1:]
				values = append(b, a...)
				found = true
				break
			}
		}

		// Update or delete the key based on whether any subscriptions remain
		if found {
			if len(values) == 0 {
				table.Delete(key)
			} else {
				table.Upsert(key, values)
			}
		}
	}
}

func (s *SubscriptionLoader) fetchAllSubscriptions(ctx context.Context) ([]datastore.Subscription, error) {
	projects, err := s.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		return []datastore.Subscription{}, nil
	}

	ids := make([]string, len(projects))
	for i := range projects {
		ids[i] = projects[i].UID
	}

	subscriptions, err := s.subRepo.LoadAllSubscriptionConfig(ctx, ids, s.batchSize)
	if err != nil {
		s.log.WithError(err).Errorf("failed to load subscriptions of all projects")
		return nil, err
	}

	return subscriptions, nil
}

func (s *SubscriptionLoader) fetchUpdatedSubscriptions(ctx context.Context) ([]datastore.Subscription, error) {
	projects, err := s.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		return []datastore.Subscription{}, nil
	}

	ids := make([]string, len(projects))
	for i := range projects {
		ids[i] = projects[i].UID
	}

	subscriptions, err := s.subRepo.FetchUpdatedSubscriptions(ctx, ids, s.subscriptionUpdates, s.batchSize)
	if err != nil {
		s.log.WithError(err).Errorf("failed to load updated subscriptions of all projects")
		return nil, err
	}

	return subscriptions, nil
}

func (s *SubscriptionLoader) fetchDeletedSubscriptions(ctx context.Context) ([]datastore.Subscription, error) {
	projects, err := s.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		return []datastore.Subscription{}, nil
	}

	ids := make([]string, len(projects))
	for i := range projects {
		ids[i] = projects[i].UID
	}

	subscriptions, err := s.subRepo.FetchDeletedSubscriptions(ctx, ids, s.subscriptionUpdates, s.batchSize)
	if err != nil {
		s.log.WithError(err).Errorf("failed to load deleted subscriptions of all projects")
		return nil, err
	}

	return subscriptions, nil
}
