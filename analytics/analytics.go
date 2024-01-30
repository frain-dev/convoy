package analytics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/frain-dev/convoy/cache"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/hibiken/asynq"
)

const (
	DailyEventCount         string = "Daily Event Count"
	DailyOrganisationCount  string = "Daily Organization Count"
	DailyProjectCount       string = "Daily Project Count"
	DailyActiveProjectCount string = "Daily Active Project Count"
	DailyUserCount          string = "Daily User Count"

	PerPage int = 50
	Page    int = 1
)

var DefaultCursor = fmt.Sprintf("%d", math.MaxInt)

type Tracker interface {
	Track() error
	Name() string
}

type Event map[string]interface{}

type AnalyticsClient interface {
	Export(eventName string, e Event) error
}

type analyticsMap map[string]Tracker

type Repo struct {
	ConfigRepo  datastore.ConfigurationRepository
	EventRepo   datastore.EventRepository
	projectRepo datastore.ProjectRepository
	OrgRepo     datastore.OrganisationRepository
	UserRepo    datastore.UserRepository
}

type Analytics struct {
	Repo       *Repo
	trackers   analyticsMap
	client     AnalyticsClient
	instanceID string
}

func newAnalytics(Repo *Repo, cfg config.Configuration) (*Analytics, error) {
	client, err := NewMixPanelClient(cfg)
	if err != nil {
		return nil, err
	}

	a := &Analytics{Repo: Repo, client: client}

	config, err := a.Repo.ConfigRepo.LoadConfiguration(context.Background())
	if err != nil {
		if errors.Is(err, datastore.ErrConfigNotFound) {
			return nil, err
		}

		log.WithError(err).Error("failed to track metrics")
		return nil, err
	}

	isEnabled := config.IsAnalyticsEnabled
	if !isEnabled {
		return nil, nil
	}

	a.instanceID = config.UID

	a.RegisterTrackers()
	return a, nil
}

func TrackDailyAnalytics(db database.Database, cache cache.Cache, cfg config.Configuration, rd *rdb.Redis) func(context.Context, *asynq.Task) error {
	repo := &Repo{
		ConfigRepo:  postgres.NewConfigRepo(db),
		EventRepo:   postgres.NewEventRepo(db, cache),
		projectRepo: postgres.NewProjectRepo(db, cache),
		OrgRepo:     postgres.NewOrgRepo(db, cache),
		UserRepo:    postgres.NewUserRepo(db, cache),
	}

	// Create a pool with go-redis
	pool := goredis.NewPool(rd.Client())
	rs := redsync.New(pool)

	// Do your work that requires the lock.

	return func(ctx context.Context, t *asynq.Task) error {
		// Obtain a new mutex by using the same name for all instances wanting the
		// same lock.
		const mutexName = "convoy:analytics:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(time.Second), redsync.WithTries(1))

		tctx, cancel := context.WithTimeout(ctx, time.Second*2)
		defer cancel()

		// Obtain a lock for our given mutex. After this is successful, no one else
		// can obtain the same lock (the same mutex name) until we unlock it.
		err := mutex.LockContext(tctx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		defer func() {
			tctx, cancel := context.WithTimeout(ctx, time.Second*2)
			defer cancel()

			// Release the lock so other processes or threads can obtain a lock.
			ok, err := mutex.UnlockContext(tctx)
			if !ok || err != nil {
				log.WithError(err).Error("failed to release lock")
			}
		}()

		a, err := newAnalytics(repo, cfg)
		if err != nil {
			log.WithError(err).Error("Failed to initialize analytics")
			return nil
		}

		if a != nil {
			a.trackDailyAnalytics()
		}
		return nil
	}
}

func (a *Analytics) trackDailyAnalytics() {
	for _, tracker := range a.trackers {
		go func(tracker Tracker) {
			err := tracker.Track()
			if err != nil {
				log.WithError(err).Error("failed to track metrics")
			}
		}(tracker)
	}
}

func (a *Analytics) RegisterTrackers() {
	a.trackers = analyticsMap{
		DailyEventCount:         newEventAnalytics(a.Repo.EventRepo, a.Repo.projectRepo, a.Repo.OrgRepo, a.client, a.instanceID),
		DailyProjectCount:       newProjectAnalytics(a.Repo.projectRepo, a.client, a.instanceID),
		DailyActiveProjectCount: newActiveProjectAnalytics(a.Repo.projectRepo, a.Repo.EventRepo, a.Repo.OrgRepo, a.client, a.instanceID),
	}
}

type NoopAnalyticsClient struct{}

func NewNoopAnalyticsClient() *NoopAnalyticsClient {
	return &NoopAnalyticsClient{}
}

func (n *NoopAnalyticsClient) Export(eventName string, e Event) error {
	return nil
}
