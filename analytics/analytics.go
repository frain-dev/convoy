package analytics

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/dukex/mixpanel"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	DailyEventCount        string = "Daily Event Count"
	DailyOrganisationCount string = "Daily Organization Count"
	DailyGroupCount        string = "Daily Project Count"
	DailyActiveGroupCount  string = "Daily Active Project Count"
	DailyUserCount         string = "Daily User Count"
	MixPanelDevToken       string = "YTAwYWI1ZWE3OTE2MzQwOWEwMjk4ZTA1NTNkNDQ0M2M="
	MixPanelProdToken      string = "YWViNzUwYWRmYjM0YTZmZjJkMzg2YTYyYWVhY2M2NWI="
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
	ConfigRepo datastore.ConfigurationRepository
	EventRepo  datastore.EventRepository
	GroupRepo  datastore.GroupRepository
	OrgRepo    datastore.OrganisationRepository
	UserRepo   datastore.UserRepository
}

type Analytics struct {
	Repo     *Repo
	trackers analyticsMap
	client   AnalyticsClient
}

func newAnalytics(Repo *Repo, cfg config.Configuration) (*Analytics, error) {
	client, err := NewMixPanelClient(cfg)
	if err != nil {
		return nil, err
	}

	a := &Analytics{Repo: Repo, client: client}

	a.RegisterTrackers()
	return a, nil
}

func TrackDailyAnalytics(Repo *Repo, cfg config.Configuration) {
	a, err := newAnalytics(Repo, cfg)
	if err != nil {
		log.Fatal(err)
	}

	a.trackDailyAnalytics()
}

func (a *Analytics) trackDailyAnalytics() {
	config, err := a.Repo.ConfigRepo.LoadConfiguration(context.Background())
	if err != nil {
		if errors.Is(err, datastore.ErrConfigNotFound) {
			return
		}

		log.WithError(err).Error("failed to track metrics")
	}

	if !config.IsAnalyticsEnabled {
		return
	}

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
		DailyEventCount:        newEventAnalytics(a.Repo.EventRepo, a.client),
		DailyOrganisationCount: newOrganisationAnalytics(a.Repo.OrgRepo, a.client),
		DailyGroupCount:        newGroupAnalytics(a.Repo.GroupRepo, a.client),
		DailyActiveGroupCount:  newActiveGroupAnalytics(a.Repo.GroupRepo, a.Repo.EventRepo, a.client),
		DailyUserCount:         newUserAnalytics(a.Repo.UserRepo, a.client),
	}

}

type MixPanelClient struct {
	client mixpanel.Mixpanel
}

func NewMixPanelClient(cfg config.Configuration) (*MixPanelClient, error) {
	token := MixPanelDevToken

	if cfg.Environment == "prod" {
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
