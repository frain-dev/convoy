package mongo

import (
	"context"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/newrelic/go-agent/v3/integrations/nrmongo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	ConfigCollection              = "configurations"
	GroupCollection               = "groups"
	OrganisationCollection        = "organisations"
	OrganisationInvitesCollection = "organisation_invites"
	OrganisationMembersCollection = "organisation_members"
	AppCollection                 = "applications"
	DeviceCollection              = "devices"
	EventCollection               = "events"
	SourceCollection              = "sources"
	UserCollection                = "users"
	SubscriptionCollection        = "subscriptions"
)

type Client struct {
	db                *mongo.Database
	apiKeyRepo        datastore.APIKeyRepository
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	applicationRepo   datastore.ApplicationRepository
	subscriptionRepo  datastore.SubscriptionRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	sourceRepo        datastore.SourceRepository
	orgRepo           datastore.OrganisationRepository
	orgMemberRepo     datastore.OrganisationMemberRepository
	orgInviteRepo     datastore.OrganisationInviteRepository
	userRepo          datastore.UserRepository
	deviceRepo        datastore.DeviceRepository
	configRepo        datastore.ConfigurationRepository
}

func New(cfg config.Configuration) (datastore.DatabaseClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := options.Client()
	newRelicMonitor := nrmongo.NewCommandMonitor(nil)
	opts.SetMonitor(newRelicMonitor)
	opts.ApplyURI(cfg.Database.Dsn)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	u, err := url.Parse(cfg.Database.Dsn)
	if err != nil {
		return nil, err
	}

	dbName := strings.TrimPrefix(u.Path, "/")
	conn := client.Database(dbName, nil)
	groups := datastore.New(conn, GroupCollection)
	events := datastore.New(conn, EventCollection)
	sources := datastore.New(conn, SourceCollection)
	apps := datastore.New(conn, AppCollection)
	subscriptions := datastore.New(conn, SubscriptionCollection)
	orgs := datastore.New(conn, OrganisationCollection)
	org_member := datastore.New(conn, OrganisationMembersCollection)
	org_invite := datastore.New(conn, OrganisationInvitesCollection)
	users := datastore.New(conn, UserCollection)
	config := datastore.New(conn, ConfigCollection)
	devices := datastore.New(conn, DeviceCollection)

	c := &Client{
		db:                conn,
		apiKeyRepo:        NewApiKeyRepo(conn),
		groupRepo:         NewGroupRepo(conn, groups),
		applicationRepo:   NewApplicationRepo(conn, apps),
		subscriptionRepo:  NewSubscriptionRepo(conn, subscriptions),
		eventRepo:         NewEventRepository(conn, events),
		eventDeliveryRepo: NewEventDeliveryRepository(conn),
		sourceRepo:        NewSourceRepo(conn, sources),
		deviceRepo:        NewDeviceRepository(conn, devices),
		orgRepo:           NewOrgRepo(conn, orgs),
		orgMemberRepo:     NewOrgMemberRepo(conn, org_member),
		orgInviteRepo:     NewOrgInviteRepo(conn, org_invite),
		userRepo:          NewUserRepo(conn, users),
		configRepo:        NewConfigRepo(conn, config),
	}

	c.ensureMongoIndices()

	return c, nil
}

func (c *Client) Disconnect(ctx context.Context) error {
	return c.db.Client().Disconnect(ctx)
}

func (c *Client) GetName() string {
	return "mongo"
}

func (c *Client) Client() interface{} {
	return c.db
}

func (c *Client) APIRepo() datastore.APIKeyRepository {
	return c.apiKeyRepo
}

func (c *Client) GroupRepo() datastore.GroupRepository {
	return c.groupRepo
}

func (c *Client) AppRepo() datastore.ApplicationRepository {
	return c.applicationRepo
}

func (c *Client) DeviceRepo() datastore.DeviceRepository {
	return c.deviceRepo
}

func (c *Client) EventRepo() datastore.EventRepository {
	return c.eventRepo
}

func (c *Client) EventDeliveryRepo() datastore.EventDeliveryRepository {
	return c.eventDeliveryRepo
}

func (c *Client) SubRepo() datastore.SubscriptionRepository {
	return c.subscriptionRepo
}

func (c *Client) SourceRepo() datastore.SourceRepository {
	return c.sourceRepo
}

func (c *Client) OrganisationRepo() datastore.OrganisationRepository {
	return c.orgRepo
}

func (c *Client) OrganisationMemberRepo() datastore.OrganisationMemberRepository {
	return c.orgMemberRepo
}

func (c *Client) OrganisationInviteRepo() datastore.OrganisationInviteRepository {
	return c.orgInviteRepo
}

func (c *Client) UserRepo() datastore.UserRepository {
	return c.userRepo
}

func (c *Client) ConfigurationRepo() datastore.ConfigurationRepository {
	return c.configRepo
}

func (c *Client) ensureMongoIndices() {
	c.ensureIndex(GroupCollection, "uid", true, nil)

	c.ensureIndex(OrganisationCollection, "uid", true, nil)

	c.ensureIndex(OrganisationMembersCollection, "organisation_id", false, nil)
	c.ensureIndex(OrganisationMembersCollection, "user_id", false, nil)
	c.ensureIndex(OrganisationMembersCollection, "uid", true, nil)

	c.ensureIndex(OrganisationInvitesCollection, "uid", true, nil)
	c.ensureIndex(OrganisationInvitesCollection, "token", true, nil)

	c.ensureIndex(AppCollection, "group_id", false, nil)
	c.ensureIndex(UserCollection, "uid", true, nil)
	c.ensureIndex(UserCollection, "reset_password_token", true, nil)
	c.ensureIndex(AppCollection, "uid", true, nil)

	c.ensureIndex(EventCollection, "uid", true, nil)
	c.ensureIndex(EventCollection, "app_id", false, nil)
	c.ensureIndex(EventCollection, "group_id", false, nil)
	c.ensureIndex(AppCollection, "group_id", false, nil)
	c.ensureIndex(EventDeliveryCollection, "status", false, nil)
	c.ensureIndex(SourceCollection, "uid", true, nil)
	c.ensureIndex(SourceCollection, "mask_id", true, nil)
	c.ensureIndex(SubscriptionCollection, "uid", true, nil)
	c.ensureIndex(SubscriptionCollection, "filter_config.event_type", false, nil)
	c.ensureCompoundIndex(AppCollection)
	c.ensureCompoundIndex(EventCollection)
	c.ensureCompoundIndex(UserCollection)
	c.ensureCompoundIndex(GroupCollection)
	c.ensureCompoundIndex(SubscriptionCollection)
	c.ensureCompoundIndex(DeviceCollection)
	c.ensureCompoundIndex(EventDeliveryCollection)
	c.ensureCompoundIndex(OrganisationInvitesCollection)
	c.ensureCompoundIndex(OrganisationMembersCollection)
}

// ensureIndex - ensures an index is created for a specific field in a collection
func (c *Client) ensureIndex(collectionName string, field string, unique bool, partialFilterExpression interface{}) bool {
	createIndexOpts := &options.IndexOptions{Unique: &unique}

	if partialFilterExpression != nil {
		createIndexOpts.SetPartialFilterExpression(partialFilterExpression)
	}

	mod := mongo.IndexModel{
		Keys:    bson.D{{Key: field, Value: 1}}, // index in ascending order or -1 for descending order
		Options: createIndexOpts,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := c.db.Collection(collectionName)

	_, err := collection.Indexes().CreateOne(ctx, mod)
	if err != nil {
		log.WithError(err).Errorf("failed to create index on field %s in %s", field, collectionName)
		return false
	}

	return true
}

func (c *Client) ensureCompoundIndex(collectionName string) bool {
	collection := c.db.Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	compoundIndices := compoundIndices()

	compoundIndex, ok := compoundIndices[collectionName]

	if !ok {
		return false
	}

	_, err := collection.Indexes().CreateMany(ctx, compoundIndex)

	if err != nil {
		log.WithError(err).Errorf("failed to create index on collection %s", collectionName)
		return false
	}

	return true
}

func compoundIndices() map[string][]mongo.IndexModel {
	compoundIndices := map[string][]mongo.IndexModel{
		GroupCollection: {
			{
				Keys: bson.D{
					{Key: "organisation_id", Value: 1},
					{Key: "name", Value: 1},
					{Key: "document_status", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},
		EventCollection: {
			{
				Keys: bson.D{
					{Key: "group_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: -1},
				},
			},

			{
				Keys: bson.D{
					{Key: "group_id", Value: 1},
					{Key: "app_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: -1},
				},
			},

			{
				Keys: bson.D{
					{Key: "app_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: -1},
				},
			},

			{
				Keys: bson.D{
					{Key: "group_id", Value: 1},
					{Key: "app_id", Value: 1},
					{Key: "created_at", Value: -1},
				},
			},

			{
				Keys: bson.D{
					{Key: "app_id", Value: 1},
					{Key: "group_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "app_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "group_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
				},
			},
		},

		EventDeliveryCollection: {
			{
				Keys: bson.D{
					{Key: "event_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "event_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
					{Key: "status", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
					{Key: "group_id", Value: 1},
					{Key: "status", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "uid", Value: 1},
					{Key: "document_status", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "group_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: -1},
					{Key: "group_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: -1},
					{Key: "app_id", Value: 1},
					{Key: "group_id", Value: 1},
				},
			},

			{
				Keys: bson.D{
					{Key: "created_at", Value: -1},
				},
			},
		},

		AppCollection: {
			{
				Keys: bson.D{
					{Key: "group_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "created_at", Value: 1},
				},
			},
			{
				Keys: bson.D{
					{Key: "group_id", Value: 1},
					{Key: "document_status", Value: 1},
					{Key: "title", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},

		OrganisationInvitesCollection: {
			{
				Keys: bson.D{
					{Key: "organisation_id", Value: 1},
					{Key: "invitee_email", Value: 1},
					{Key: "document_status", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
			{
				Keys: bson.D{
					{Key: "token", Value: 1},
					{Key: "email", Value: 1},
					{Key: "document_status", Value: 1},
				},
			},
		},

		OrganisationMembersCollection: {
			{
				Keys: bson.D{
					{Key: "organisation_id", Value: 1},
					{Key: "user_id", Value: 1},
					{Key: "document_status", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},

		UserCollection: {
			{
				Keys: bson.D{
					{Key: "email", Value: 1},
					{Key: "document_status", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},

		DeviceCollection: {
			{
				Keys: bson.D{
					{Key: "app_id", Value: 1},
					{Key: "group_id", Value: 1},
					{Key: "document_status", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},

		SubscriptionCollection: {
			{
				Keys: bson.D{
					{Key: "app_id", Value: 1},
					{Key: "group_id", Value: 1},
					{Key: "source_id", Value: 1},
					{Key: "endpoint_id", Value: 1},
					{Key: "document_status", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
			{
				Keys: bson.D{
					{Key: "device_id", Value: 1},
					{Key: "app_id", Value: 1},
					{Key: "group_id", Value: 1},
					{Key: "source_id", Value: 1},
					{Key: "document_status", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		},
	}

	return compoundIndices
}
