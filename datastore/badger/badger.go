package badger

import (
	"context"
	"io"

	"github.com/frain-dev/convoy/util"

	"github.com/dgraph-io/badger/v3"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/sirupsen/logrus"
	"github.com/timshannon/badgerhold/v4"
)

type Client struct {
	store             *badgerhold.Store
	apiKeyRepo        datastore.APIKeyRepository
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	subRepo           datastore.SubscriptionRepository
	applicationRepo   datastore.ApplicationRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	sourceRepo        datastore.SourceRepository
	orgRepo           datastore.OrganisationRepository
	orgMemberRepo     datastore.OrganisationMemberRepository
	orgInviteRepo     datastore.OrganisationInviteRepository
	userRepo          datastore.UserRepository
	configRepo        datastore.ConfigurationRepository
}

func (c *Client) OrganisationMemberRepo() datastore.OrganisationMemberRepository {
	return c.orgMemberRepo
}

func (c *Client) OrganisationInviteRepo() datastore.OrganisationInviteRepository {
	return c.orgInviteRepo
}

func New(cfg config.Configuration) (datastore.DatabaseClient, error) {
	dsn := cfg.Database.Dsn
	if util.IsStringEmpty(dsn) {
		dsn = "./convoy_db"
	}

	st, err := badgerhold.Open(badgerhold.Options{
		Encoder:          badgerhold.DefaultEncode,
		Decoder:          badgerhold.DefaultDecode,
		SequenceBandwith: 100,
		Options: badger.DefaultOptions(dsn).
			WithZSTDCompressionLevel(0).
			WithCompression(0).WithLogger(&logrus.Logger{Out: io.Discard}),
	})

	if err != nil {
		return nil, err
	}

	c := &Client{
		store:             st,
		groupRepo:         NewGroupRepo(st),
		eventRepo:         NewEventRepo(st),
		apiKeyRepo:        NewApiRoleRepo(st),
		applicationRepo:   NewApplicationRepo(st),
		subRepo:           NewSubscriptionRepo(st),
		eventDeliveryRepo: NewEventDeliveryRepository(st),
		sourceRepo:        NewSourceRepo(st),
		orgRepo:           NewOrgRepo(st),
		orgMemberRepo:     NewOrgMemberRepo(st),
		orgInviteRepo:     NewOrgInviteRepo(st),
		userRepo:          NewUserRepo(st),
		configRepo:        NewConfigRepo(st),
	}

	return c, nil
}

func (c *Client) Disconnect(context.Context) error {
	return c.store.Close()
}

func (c *Client) GetName() string {
	return "badger"
}

func (c *Client) Client() interface{} {
	return c.store
}

func (c *Client) GroupRepo() datastore.GroupRepository {
	return c.groupRepo
}

func (c *Client) AppRepo() datastore.ApplicationRepository {
	return c.applicationRepo
}

func (c *Client) EventRepo() datastore.EventRepository {
	return c.eventRepo
}

func (c *Client) EventDeliveryRepo() datastore.EventDeliveryRepository {
	return c.eventDeliveryRepo
}

func (c *Client) APIRepo() datastore.APIKeyRepository {
	return c.apiKeyRepo
}

func (c *Client) SubRepo() datastore.SubscriptionRepository {
	return c.subRepo
}

func (c *Client) SourceRepo() datastore.SourceRepository {
	return c.sourceRepo
}

func (c *Client) OrganisationRepo() datastore.OrganisationRepository {
	return c.orgRepo
}

func (c *Client) UserRepo() datastore.UserRepository {
	return c.userRepo
}

func (c *Client) ConfigurationRepo() datastore.ConfigurationRepository {
	return c.configRepo
}
