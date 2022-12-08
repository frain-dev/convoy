package analytics

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/dukex/mixpanel"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	DailyEventCount         string = "Daily Event Count"
	DailyOrganisationCount  string = "Daily Organization Count"
	DailyProjectCount       string = "Daily Project Count"
	DailyActiveProjectCount string = "Daily Active Project Count"
	DailyUserCount          string = "Daily User Count"
	MixPanelDevToken        string = "YTAwYWI1ZWE3OTE2MzQwOWEwMjk4ZTA1NTNkNDQ0M2M="
	MixPanelProdToken       string = "YWViNzUwYWRmYjM0YTZmZjJkMzg2YTYyYWVhY2M2NWI="
	PerPage                 int    = 50
	Page                    int    = 1
)

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

	if !config.IsAnalyticsEnabled {
		return nil, nil
	}

	a.instanceID = config.UID

	a.RegisterTrackers()
	return a, nil
}

func TrackDailyAnalytics(store datastore.Store, cfg config.Configuration) func(context.Context, *asynq.Task) error {
	repo := &Repo{
		ConfigRepo:  cm.NewConfigRepo(store),
		EventRepo:   cm.NewEventRepository(store),
		projectRepo: cm.NewProjectRepo(store),
		OrgRepo:     cm.NewOrgRepo(store),
		UserRepo:    cm.NewUserRepo(store),
	}
	return func(ctx context.Context, t *asynq.Task) error {
		a, err := newAnalytics(repo, cfg)
		if err != nil {
			log.WithError(err).Error("Failed to initialize analytics")
			return nil
		}

		a.trackDailyAnalytics()

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
		DailyOrganisationCount:  newOrganisationAnalytics(a.Repo.OrgRepo, a.client, a.instanceID),
		DailyProjectCount:       newProjectAnalytics(a.Repo.projectRepo, a.client, a.instanceID),
		DailyActiveProjectCount: newActiveProjectAnalytics(a.Repo.projectRepo, a.Repo.EventRepo, a.Repo.OrgRepo, a.client, a.instanceID),
		DailyUserCount:          newUserAnalytics(a.Repo.UserRepo, a.client, a.instanceID),
	}
}

type MixPanelClient struct {
	client mixpanel.Mixpanel
}

func NewMixPanelClient(cfg config.Configuration) (*MixPanelClient, error) {
	token := MixPanelDevToken

	if cfg.Environment == "cloud" {
		token = MixPanelProdToken
	}

	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}

	c := mixpanel.New(string(decoded), "")
	return &MixPanelClient{client: c}, nil
}

func (m *MixPanelClient) Export(eventName string, e Event) error {
	err := m.client.Track(uuid.NewString(), eventName, &mixpanel.Event{
		IP:         "0",
		Timestamp:  nil,
		Properties: e,
	})
	if err != nil {
		return err
	}

	return nil
}

type NoopAnalyticsClient struct{}

func NewNoopAnalyticsClient() *NoopAnalyticsClient {
	return &NoopAnalyticsClient{}
}

func (n *NoopAnalyticsClient) Export(eventName string, e Event) error {
	return nil
}
