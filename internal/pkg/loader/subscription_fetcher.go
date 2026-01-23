package loader

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

// subscriptionFetcher implements SubscriptionFetcher
type subscriptionFetcher struct {
	subRepo                datastore.SubscriptionRepository
	projectExtractor       *ProjectIDExtractor
	batchSize              int64
	subscriptionCollection *SubscriptionCollection
	lastSyncTime           time.Time
	log                    log.StdLogger
}

// NewSubscriptionFetcher creates a new subscription fetcher
func NewSubscriptionFetcher(
	subRepo datastore.SubscriptionRepository,
	projectRepo datastore.ProjectRepository,
	subscriptionCollection *SubscriptionCollection,
	log log.StdLogger,
	batchSize int64,
) SubscriptionFetcher {
	return &subscriptionFetcher{
		subRepo:                subRepo,
		projectExtractor:       NewProjectIDExtractor(projectRepo),
		subscriptionCollection: subscriptionCollection,
		log:                    log,
		batchSize:              batchSize,
	}
}

// FetchAllSubscriptions fetches all subscriptions for all projects
func (sf *subscriptionFetcher) FetchAllSubscriptions(ctx context.Context) ([]datastore.Subscription, error) {
	projectIDs, err := sf.projectExtractor.ExtractProjectIDs(ctx)
	if err != nil {
		return nil, err
	}

	if len(projectIDs) == 0 {
		return []datastore.Subscription{}, nil
	}

	subscriptions, err := sf.subRepo.LoadAllSubscriptionConfig(ctx, projectIDs, sf.batchSize)
	if err != nil {
		sf.log.WithError(err).Error("failed to load subscriptions of all projects")
		return nil, err
	}

	return subscriptions, nil
}

// FetchUpdatedSubscriptions fetches updated subscriptions based on the current collection
func (sf *subscriptionFetcher) FetchUpdatedSubscriptions(ctx context.Context) ([]datastore.Subscription, error) {
	projectIDs, err := sf.projectExtractor.ExtractProjectIDs(ctx)
	if err != nil {
		return nil, err
	}

	if len(projectIDs) == 0 {
		return []datastore.Subscription{}, nil
	}

	subscriptions, err := sf.subRepo.FetchUpdatedSubscriptions(
		ctx,
		projectIDs,
		sf.subscriptionCollection.GetAll(),
		sf.batchSize,
	)
	if err != nil {
		sf.log.WithError(err).Error("failed to load updated subscriptions of all projects")
		return nil, err
	}

	return subscriptions, nil
}

// FetchNewSubscriptions fetches newly created subscriptions since last sync
func (sf *subscriptionFetcher) FetchNewSubscriptions(ctx context.Context) ([]datastore.Subscription, error) {
	projectIDs, err := sf.projectExtractor.ExtractProjectIDs(ctx)
	if err != nil {
		return nil, err
	}

	if len(projectIDs) == 0 {
		return []datastore.Subscription{}, nil
	}

	// Get known subscription IDs from the collection
	knownSubs := sf.subscriptionCollection.GetAll()
	knownIDs := make([]string, len(knownSubs))
	for i, sub := range knownSubs {
		knownIDs[i] = sub.UID
	}

	subscriptions, err := sf.subRepo.FetchNewSubscriptions(
		ctx,
		projectIDs,
		knownIDs,
		sf.lastSyncTime,
		sf.batchSize,
	)
	if err != nil {
		sf.log.WithError(err).Error("failed to load new subscriptions of all projects")
		return nil, err
	}

	return subscriptions, nil
}

// SetLastSyncTime updates the last sync time for fetching new subscriptions
func (sf *subscriptionFetcher) SetLastSyncTime(t time.Time) {
	sf.lastSyncTime = t
}

// FetchDeletedSubscriptions fetches deleted subscriptions based on the current collection
func (sf *subscriptionFetcher) FetchDeletedSubscriptions(ctx context.Context) ([]datastore.Subscription, error) {
	projectIDs, err := sf.projectExtractor.ExtractProjectIDs(ctx)
	if err != nil {
		return nil, err
	}

	if len(projectIDs) == 0 {
		return []datastore.Subscription{}, nil
	}

	subscriptions, err := sf.subRepo.FetchDeletedSubscriptions(
		ctx,
		projectIDs,
		sf.subscriptionCollection.GetAll(),
		sf.batchSize,
	)
	if err != nil {
		sf.log.WithError(err).Error("failed to load deleted subscriptions of all projects")
		return nil, err
	}

	return subscriptions, nil
}
