package loader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

const (
	perPage = 1000
)

type SubscriptionLoader struct {
	subRepo     datastore.SubscriptionRepository
	projectRepo datastore.ProjectRepository

	log log.StdLogger
}

func NewSubscriptionLoader(subRepo datastore.SubscriptionRepository, projectRepo datastore.ProjectRepository, log log.StdLogger) *SubscriptionLoader {
	return &SubscriptionLoader{
		log:         log,
		subRepo:     subRepo,
		projectRepo: projectRepo,
	}
}

func (s *SubscriptionLoader) SyncChanges(ctx context.Context, table *memorystore.Table) error {
	mSubKeys := table.GetKeys()

	subscriptions, err := s.fetchSubscriptions(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to fetch subscriptions")
		return err
	}

	var dSubKeys []string
	for _, sub := range subscriptions {
		key, err := s.generateSubKey(sub)
		if err != nil {
			return err
		}
		dSubKeys = append(dSubKeys, key)
	}

	// find new and updated rows
	newRows := util.Difference(dSubKeys, mSubKeys)
	if len(newRows) != 0 {
		for _, idx := range newRows {
			for _, sub := range subscriptions {
				key, err := s.generateSubKey(sub)
				if err != nil {
					return err
				}

				if key == idx {
					_ = table.Add(idx, sub)
				}
			}
		}
	}

	// find deleted rows
	deletedRows := util.Difference(mSubKeys, dSubKeys)
	if len(deletedRows) != 0 {
		for _, idx := range deletedRows {
			table.Delete(idx)
		}
	}

	return nil
}

func (s *SubscriptionLoader) fetchSubscriptions(ctx context.Context) ([]*datastore.Subscription, error) {
	projects, err := s.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	resultChan := make(chan []*datastore.Subscription, len(projects))

	for i := range projects {
		wg.Add(1)

		go func(projectID string) {
			defer wg.Done()

			var subscriptions []*datastore.Subscription
			subscriptions, err = s.fetchSubscriptionBatch(ctx, subscriptions, projectID, "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
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

	var allSubscriptions []*datastore.Subscription
	for projectSubs := range resultChan {
		allSubscriptions = append(allSubscriptions, projectSubs...)
	}

	return allSubscriptions, nil
}

func (s *SubscriptionLoader) fetchSubscriptionBatch(ctx context.Context,
	subscriptions []*datastore.Subscription, projectID string, cursor string,
) ([]*datastore.Subscription, error) {
	pageable := datastore.Pageable{
		NextCursor: cursor,
		Direction:  datastore.Next,
		PerPage:    perPage,
	}

	newSubscriptions, pagination, err := s.subRepo.LoadSubscriptionsPaged(ctx, projectID, &datastore.FilterBy{}, pageable)
	if err != nil {
		return nil, err
	}

	if len(newSubscriptions) == 0 && !pagination.HasNextPage {
		return subscriptions, nil
	}

	if pagination.HasNextPage {
		cursor = pagination.NextPageCursor
		subscriptions = append(subscriptions, newSubscriptions...)
		return s.fetchSubscriptionBatch(ctx, subscriptions, projectID, cursor)
	}

	subscriptions = append(subscriptions, newSubscriptions...)
	return subscriptions, nil
}

func (s *SubscriptionLoader) generateSubKey(sub *datastore.Subscription) (string, error) {
	var hash string

	bytes, err := json.Marshal(sub)
	if err != nil {
		return hash, err
	}

	sha256Hash := sha256.New()
	sha256Hash.Write(bytes)
	hashBytes := sha256Hash.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return hashString, nil
}
