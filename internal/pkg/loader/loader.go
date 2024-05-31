package loader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/pkg/log"
)

const (
	perPage = 1000
)

type SubscriptionLoader struct {
	once sync.Once

	subRepo     datastore.SubscriptionRepository
	projectRepo datastore.ProjectRepository

	loaded   bool
	lastSync time.Time
	log      log.StdLogger
}

func NewSubscriptionLoader(subRepo datastore.SubscriptionRepository, projectRepo datastore.ProjectRepository, log log.StdLogger) *SubscriptionLoader {
	return &SubscriptionLoader{
		log:         log,
		subRepo:     subRepo,
		projectRepo: projectRepo,
	}
}

func (s *SubscriptionLoader) SyncChanges(ctx context.Context, table *memorystore.Table) error {

	if !s.loaded {
		// fetch subscriptions.
		subscriptions, err := s.fetchAllSubscriptions(ctx)
		if err != nil {
			s.log.WithError(err).Error("failed to fetch subscriptions")
			return err
		}

		for _, sub := range subscriptions {
			key, err := s.generateSubKey(&sub)
			if err != nil {
				return err
			}
			table.Add(key, sub)
		}

		s.loaded = true
		s.lastSync = time.Now()

		return nil
	}

	// fetch subscriptions.
	updatedSubs, err := s.fetchUpdatedSubscriptions(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to fetch subscriptions")
		return err
	}
	fmt.Println(updatedSubs)

	if len(updatedSubs) != 0 {
		for _, sub := range updatedSubs {
			key, err := s.generateSubKey(&sub)
			if err != nil {
				return err
			}

			_ = table.Upsert(key, sub)
		}
	}

	// fetch subscriptions.
	deletedSubs, err := s.fetchDeletedSubscriptions(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to fetch subscriptions")
		return err
	}
	fmt.Println(deletedSubs)

	for _, sub := range deletedSubs {
		key, err := s.generateSubKey(&sub)
		if err != nil {
			return err
		}

		table.Delete(key)
	}

	s.lastSync = time.Now()

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
			subscriptions, err := s.subRepo.LoadAllSubscriptionConfig(ctx, projectID, 10_000)
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
			subscriptions, err := s.subRepo.FetchUpdatedSubscriptions(ctx, projectID, s.lastSync, 10_000)
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
			subscriptions, err := s.subRepo.FetchDeletedSubscriptions(ctx, projectID, s.lastSync, 10_000)
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
