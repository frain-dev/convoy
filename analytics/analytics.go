package analytics

import (
	"encoding/base64"

	"github.com/dukex/mixpanel"
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
	MixPanelToken          string = "N2ViYzQ4ZTc4NWMwNmQ5YmYyNjZhYjg3NDZiNmMxNzM="
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
	EventRepo datastore.EventRepository
	GroupRepo datastore.GroupRepository
	OrgRepo   datastore.OrganisationRepository
	UserRepo  datastore.UserRepository
}

type Analytics struct {
	Repo     *Repo
	trackers analyticsMap
	client   AnalyticsClient
}

func NewAnalytics(Repo *Repo) (*Analytics, error) {
	client, err := NewMixPanelClient()
	if err != nil {
		return nil, err
	}

	a := &Analytics{Repo: Repo, client: client}

	a.RegisterTrackers()
	return a, nil
}

func (a *Analytics) TrackDailyAnalytics() {
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
		DailyEventCount:        NewEventAnalytics(a.Repo.EventRepo, a.client),
		DailyOrganisationCount: NewOrganisationAnalytics(a.Repo.OrgRepo, a.client),
		DailyGroupCount:        NewGroupAnalytics(a.Repo.GroupRepo, a.client),
		DailyActiveGroupCount:  NewActiveGroupAnalytics(a.Repo.GroupRepo, a.Repo.EventRepo, a.client),
		DailyUserCount:         NewUserAnalytics(a.Repo.UserRepo, a.client),
	}

}

type MixPanelClient struct {
	client mixpanel.Mixpanel
}

func NewMixPanelClient() (*MixPanelClient, error) {
	decoded, err := base64.StdEncoding.DecodeString(MixPanelToken)
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
