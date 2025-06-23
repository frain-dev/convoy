package loader

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/pkg/log"
)

type SubscriptionLoader struct {
	tableManager           SubscriptionTableManager
	subscriptionFetcher    SubscriptionFetcher
	subscriptionCollection *SubscriptionCollection
	config                 *LoaderConfig
	loaded                 bool
	log                    log.StdLogger
}

func NewSubscriptionLoader(
	subRepo datastore.SubscriptionRepository,
	projectRepo datastore.ProjectRepository,
	log log.StdLogger,
	batchSize int64,
) *SubscriptionLoader {
	config := NewLoaderConfig(batchSize, false)
	subscriptionCollection := NewSubscriptionCollection()

	return &SubscriptionLoader{
		tableManager:           NewSubscriptionTableManager(log),
		subscriptionFetcher:    NewSubscriptionFetcher(subRepo, projectRepo, subscriptionCollection, log, config.BatchSize),
		subscriptionCollection: subscriptionCollection,
		config:                 config,
		log:                    log,
	}
}

func (s *SubscriptionLoader) SyncChanges(ctx context.Context, table *memorystore.Table) error {
	if !s.loaded {
		return s.performInitialLoad(ctx, table)
	}

	return s.performIncrementalSync(ctx, table)
}

func (s *SubscriptionLoader) performInitialLoad(ctx context.Context, table *memorystore.Table) error {
	startTime := time.Now()

	subscriptions, err := s.subscriptionFetcher.FetchAllSubscriptions(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to fetch all subscriptions")
		return err
	}

	for _, sub := range subscriptions {
		s.subscriptionCollection.AddOrUpdate(sub)
		s.tableManager.AddSubscription(sub, table)
	}

	s.loaded = true
	s.log.Infof("syncing subscriptions completed in %fs", time.Since(startTime).Seconds())
	return nil
}

func (s *SubscriptionLoader) performIncrementalSync(ctx context.Context, table *memorystore.Table) error {
	if s.config.EnableDebug {
		s.tableManager.(*subscriptionTableManager).DebugTableContents(table)
	}

	if err := s.processUpdatedSubscriptions(ctx, table); err != nil {
		return err
	}

	if err := s.processDeletedSubscriptions(ctx, table); err != nil {
		return err
	}

	return nil
}

func (s *SubscriptionLoader) processUpdatedSubscriptions(ctx context.Context, table *memorystore.Table) error {
	updatedSubs, err := s.subscriptionFetcher.FetchUpdatedSubscriptions(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to fetch updated subscriptions")
		return err
	}

	for _, sub := range updatedSubs {
		s.tableManager.RemoveSubscriptionFromAllEventTypes(sub, table)
		s.subscriptionCollection.AddOrUpdate(sub)
		s.tableManager.AddSubscription(sub, table)
	}

	return nil
}

func (s *SubscriptionLoader) processDeletedSubscriptions(ctx context.Context, table *memorystore.Table) error {
	deletedSubs, err := s.subscriptionFetcher.FetchDeletedSubscriptions(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to fetch deleted subscriptions")
		return err
	}

	for _, sub := range deletedSubs {
		s.tableManager.RemoveSubscriptionFromAllEventTypes(sub, table)
		s.subscriptionCollection.Remove(sub.UID)
		s.tableManager.RemoveSubscription(sub, table)
	}

	return nil
}
