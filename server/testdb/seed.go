package testdb

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"github.com/xdg-go/pbkdf2"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// SeedEndpoint creates a random endpoint for integration tests.
func SeedEndpoint(store datastore.Store, g *datastore.Group, uid, title string, disabled bool) (*datastore.Endpoint, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	if util.IsStringEmpty(title) {
		title = fmt.Sprintf("TestEndpoint-%s", uid)
	}

	endpoint := &datastore.Endpoint{
		UID:            uid,
		Title:          title,
		GroupID:        g.UID,
		IsDisabled:     disabled,
		AppID:          uid,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	// Seed Data.
	endpointRepo := cm.NewEndpointRepo(store)
	err := endpointRepo.CreateEndpoint(context.TODO(), endpoint, g.UID)
	if err != nil {
		return &datastore.Endpoint{}, err
	}

	return endpoint, nil
}

func SeedMultipleEndpoints(store datastore.Store, g *datastore.Group, count int) error {
	for i := 0; i < count; i++ {
		uid := uuid.New().String()
		app := &datastore.Endpoint{
			UID:            uid,
			Title:          fmt.Sprintf("Test-%s", uid),
			GroupID:        g.UID,
			IsDisabled:     false,
			DocumentStatus: datastore.ActiveDocumentStatus,
		}

		// Seed Data.
		appRepo := cm.NewEndpointRepo(store)
		err := appRepo.CreateEndpoint(context.TODO(), app, app.GroupID)
		if err != nil {
			return err
		}
	}
	return nil
}

func SeedEndpointSecret(store datastore.Store, e *datastore.Endpoint, value string) (*datastore.Secret, error) {
	sc := datastore.Secret{
		UID:            uuid.New().String(),
		Value:          value,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	e.Secrets = append(e.Secrets, sc)

	// Seed Data.
	endpointRepo := cm.NewEndpointRepo(store)
	err := endpointRepo.UpdateEndpoint(context.TODO(), e, e.GroupID)
	if err != nil {
		return nil, err
	}

	return &sc, nil
}

// seed default group
func SeedDefaultGroup(store datastore.Store, orgID string) (*datastore.Group, error) {
	if orgID == "" {
		orgID = uuid.NewString()
	}

	defaultGroup := &datastore.Group{
		UID:            uuid.New().String(),
		Name:           "default-group",
		Type:           datastore.OutgoingGroup,
		OrganisationID: orgID,
		Config: &datastore.GroupConfig{
			Strategy: &datastore.StrategyConfiguration{
				Type:       datastore.DefaultStrategyProvider,
				Duration:   10,
				RetryCount: 2,
			},
			Signature: &datastore.SignatureConfiguration{
				Header: config.DefaultSignatureHeader,
				Versions: []datastore.SignatureVersion{
					{
						UID:       uuid.NewString(),
						Hash:      "SHA256",
						Encoding:  datastore.HexEncoding,
						CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
					},
				},
			},
			DisableEndpoint: false,
			ReplayAttacks:   false,
		},
		RateLimit:         convoy.RATE_LIMIT,
		RateLimitDuration: convoy.RATE_LIMIT_DURATION,
		CreatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus:    datastore.ActiveDocumentStatus,
	}

	// Seed Data.
	groupRepo := cm.NewGroupRepo(store)
	err := groupRepo.CreateGroup(context.TODO(), defaultGroup)
	if err != nil {
		return &datastore.Group{}, err
	}

	return defaultGroup, nil
}

const DefaultUserPassword = "password"

// seed default user
func SeedDefaultUser(store datastore.Store) (*datastore.User, error) {
	p := datastore.Password{Plaintext: DefaultUserPassword}
	err := p.GenerateHash()
	if err != nil {
		return nil, err
	}

	defaultUser := &datastore.User{
		UID:            uuid.NewString(),
		FirstName:      "default",
		LastName:       "default",
		Email:          "default@user.com",
		Password:       string(p.Hash),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	// Seed Data.
	userRepo := cm.NewUserRepo(store)
	err = userRepo.CreateUser(context.TODO(), defaultUser)
	if err != nil {
		return &datastore.User{}, err
	}

	return defaultUser, nil
}

// seed default organisation
func SeedDefaultOrganisation(store datastore.Store, user *datastore.User) (*datastore.Organisation, error) {
	p := datastore.Password{Plaintext: DefaultUserPassword}
	err := p.GenerateHash()
	if err != nil {
		return nil, err
	}

	defaultOrg := &datastore.Organisation{
		UID:            uuid.NewString(),
		OwnerID:        user.UID,
		Name:           "default-org",
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	// Seed Data.
	organisationRepo := cm.NewOrgRepo(store)
	err = organisationRepo.CreateOrganisation(context.TODO(), defaultOrg)
	if err != nil {
		return &datastore.Organisation{}, err
	}

	member := &datastore.OrganisationMember{
		UID:            uuid.NewString(),
		OrganisationID: defaultOrg.UID,
		UserID:         user.UID,
		Role:           auth.Role{Type: auth.RoleSuperUser},
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	orgMemberRepo := cm.NewOrgMemberRepo(store)
	err = orgMemberRepo.CreateOrganisationMember(context.TODO(), member)
	if err != nil {
		return nil, err
	}

	return defaultOrg, nil
}

// seed organisation member
func SeedOrganisationMember(store datastore.Store, org *datastore.Organisation, user *datastore.User, role *auth.Role) (*datastore.OrganisationMember, error) {
	member := &datastore.OrganisationMember{
		UID:            uuid.NewString(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           *role,
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	orgMemberRepo := cm.NewOrgMemberRepo(store)
	err := orgMemberRepo.CreateOrganisationMember(context.TODO(), member)
	if err != nil {
		return nil, err
	}

	return member, nil
}

// seed organisation invite
func SeedOrganisationInvite(store datastore.Store, org *datastore.Organisation, email string, role *auth.Role, expiry primitive.DateTime, status datastore.InviteStatus) (*datastore.OrganisationInvite, error) {
	if expiry == 0 {
		expiry = primitive.NewDateTimeFromTime(time.Now())
	}

	iv := &datastore.OrganisationInvite{
		UID:            uuid.NewString(),
		InviteeEmail:   email,
		OrganisationID: org.UID,
		Role:           *role,
		Token:          uniuri.NewLen(64),
		ExpiresAt:      expiry,
		Status:         status,
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	orgInviteRepo := cm.NewOrgInviteRepo(store)
	err := orgInviteRepo.CreateOrganisationInvite(context.TODO(), iv)
	if err != nil {
		return nil, err
	}

	return iv, nil
}

// SeedAPIKey creates random api key for integration tests.
func SeedAPIKey(store datastore.Store, role auth.Role, uid, name, keyType, userID string) (*datastore.APIKey, string, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	maskID, key := util.GenerateAPIKey()
	salt, err := util.GenerateSecret()
	if err != nil {
		return nil, "", errors.New("failed to generate salt")
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	apiKey := &datastore.APIKey{
		UID:            uid,
		MaskID:         maskID,
		Name:           name,
		UserID:         userID,
		Type:           datastore.KeyType(keyType),
		Role:           role,
		Hash:           encodedKey,
		Salt:           salt,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	apiRepo := cm.NewApiKeyRepo(store)
	err = apiRepo.CreateAPIKey(context.Background(), apiKey)
	if err != nil {
		return nil, "", err
	}

	return apiKey, key, nil
}

// seed default group
func SeedGroup(store datastore.Store, uid, name, orgID string, groupType datastore.GroupType, cfg *datastore.GroupConfig) (*datastore.Group, error) {
	if orgID == "" {
		orgID = uuid.NewString()
	}
	g := &datastore.Group{
		UID:               uid,
		Name:              name,
		Type:              groupType,
		Config:            cfg,
		OrganisationID:    orgID,
		RateLimit:         convoy.RATE_LIMIT,
		RateLimitDuration: convoy.RATE_LIMIT_DURATION,
		CreatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus:    datastore.ActiveDocumentStatus,
	}

	// Seed Data.
	groupRepo := cm.NewGroupRepo(store)
	err := groupRepo.CreateGroup(context.TODO(), g)
	if err != nil {
		return &datastore.Group{}, err
	}

	return g, nil
}

// SeedEvent creates a random event for integration tests.
func SeedEvent(store datastore.Store, endpoint *datastore.Endpoint, groupID string, uid, eventType string, sourceID string, data []byte) (*datastore.Event, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	ev := &datastore.Event{
		UID:            uid,
		EventType:      datastore.EventType(eventType),
		Data:           data,
		EndpointID:     endpoint.UID,
		GroupID:        groupID,
		SourceID:       sourceID,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	// Seed Data.
	eventRepo := cm.NewEventRepository(store)
	err := eventRepo.CreateEvent(context.TODO(), ev)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

// SeedEventDelivery creates a random event delivery for integration tests.
func SeedEventDelivery(store datastore.Store, event *datastore.Event, endpoint *datastore.Endpoint, groupID string, uid string, status datastore.EventDeliveryStatus, subcription *datastore.Subscription) (*datastore.EventDelivery, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	eventDelivery := &datastore.EventDelivery{
		UID:            uid,
		EventID:        event.UID,
		EndpointID:     endpoint.UID,
		Status:         status,
		SubscriptionID: subcription.UID,
		GroupID:        groupID,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	// Seed Data.
	eventDeliveryRepo := cm.NewEventDeliveryRepository(store)
	err := eventDeliveryRepo.CreateEventDelivery(context.TODO(), eventDelivery)
	if err != nil {
		return nil, err
	}

	return eventDelivery, nil
}

// SeedOrganisation is create random Organisation for integration tests.
func SeedOrganisation(store datastore.Store, uid, ownerID, name string) (*datastore.Organisation, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	if util.IsStringEmpty(name) {
		name = fmt.Sprintf("TestOrg-%s", uid)
	}

	org := &datastore.Organisation{
		UID:            uid,
		OwnerID:        ownerID,
		Name:           name,
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	// Seed Data.
	orgRepo := cm.NewOrgRepo(store)
	err := orgRepo.CreateOrganisation(context.TODO(), org)
	if err != nil {
		return &datastore.Organisation{}, err
	}

	return org, nil
}

// SeedMultipleOrganisations is creates random Organisations for integration tests.
func SeedMultipleOrganisations(store datastore.Store, ownerID string, num int) ([]*datastore.Organisation, error) {
	orgs := []*datastore.Organisation{}

	for i := 0; i < num; i++ {
		uid := uuid.New().String()

		org := &datastore.Organisation{
			UID:            uid,
			OwnerID:        ownerID,
			Name:           fmt.Sprintf("TestOrg-%s", uid),
			DocumentStatus: datastore.ActiveDocumentStatus,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		}
		orgs = append(orgs, org)

		// Seed Data.
		orgRepo := cm.NewOrgRepo(store)
		err := orgRepo.CreateOrganisation(context.TODO(), org)
		if err != nil {
			return nil, err
		}
	}

	return orgs, nil
}

func SeedSource(store datastore.Store, g *datastore.Group, uid, maskID, ds string, v *datastore.VerifierConfig) (*datastore.Source, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	if util.IsStringEmpty(maskID) {
		maskID = uuid.New().String()
	}

	if v == nil {
		v = &datastore.VerifierConfig{
			Type: datastore.HMacVerifier,
			HMac: &datastore.HMac{
				Header: "X-Convoy-Header",
				Hash:   "SHA512",
				Secret: "Convoy-Secret",
			},
		}
	}

	if util.IsStringEmpty(ds) {
		ds = "http"
	}

	source := &datastore.Source{
		UID:            uid,
		GroupID:        g.UID,
		MaskID:         maskID,
		Name:           "Convoy-Prod",
		Type:           datastore.SourceType(ds),
		Verifier:       v,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	// Seed Data
	sourceRepo := cm.NewSourceRepo(store)
	err := sourceRepo.CreateSource(context.TODO(), source)
	if err != nil {
		return nil, err
	}

	return source, nil
}

func SeedSubscription(store datastore.Store,
	g *datastore.Group,
	uid string,
	groupType datastore.GroupType,
	source *datastore.Source,
	endpoint *datastore.Endpoint,
	retryConfig *datastore.RetryConfiguration,
	alertConfig *datastore.AlertConfiguration,
	filterConfig *datastore.FilterConfiguration,
	status datastore.SubscriptionStatus,
) (*datastore.Subscription, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	if status == "" {
		status = datastore.ActiveSubscriptionStatus
	}

	subscription := &datastore.Subscription{
		UID:        uid,
		GroupID:    g.UID,
		Name:       "",
		Type:       datastore.SubscriptionTypeAPI,
		SourceID:   source.UID,
		EndpointID: endpoint.UID,

		RetryConfig:  retryConfig,
		AlertConfig:  alertConfig,
		FilterConfig: filterConfig,

		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),

		Status:         status,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	subRepo := cm.NewSubscriptionRepo(store)
	err := subRepo.CreateSubscription(context.TODO(), g.UID, subscription)
	if err != nil {
		return nil, err
	}

	return subscription, nil
}

func SeedUser(store datastore.Store, email, password string) (*datastore.User, error) {
	p := &datastore.Password{Plaintext: password}
	err := p.GenerateHash()
	if err != nil {
		return nil, err
	}

	if email == "" {
		email = "test@test.com"
	}

	user := &datastore.User{
		UID:            uuid.NewString(),
		FirstName:      "test",
		LastName:       "test",
		Password:       string(p.Hash),
		Email:          email,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	// Seed Data
	userRepo := cm.NewUserRepo(store)
	err = userRepo.CreateUser(context.TODO(), user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func SeedConfiguration(store datastore.Store) (*datastore.Configuration, error) {
	config := &datastore.Configuration{
		UID:                uuid.NewString(),
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    true,
		StoragePolicy:      &datastore.DefaultStoragePolicy,
		DocumentStatus:     datastore.ActiveDocumentStatus,
	}

	// Seed Data
	configRepo := cm.NewConfigRepo(store)
	err := configRepo.CreateConfiguration(context.TODO(), config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func SeedDevice(store datastore.Store, g *datastore.Group, endpointID string) error {
	device := &datastore.Device{
		UID:            uuid.NewString(),
		GroupID:        g.UID,
		EndpointID:     endpointID,
		HostName:       "",
		Status:         datastore.DeviceStatusOnline,
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	deviceRepo := cm.NewDeviceRepository(store)
	err := deviceRepo.CreateDevice(context.TODO(), device)
	if err != nil {
		return err
	}

	return nil
}

// PurgeDB is run after every test run and it's used to truncate the DB to have
// a clean slate in the next run.
func PurgeDB(t *testing.T, db cm.Client) {
	client := db.Client().(*mongo.Database)
	err := client.Drop(context.TODO())
	if err != nil {
		t.Fatalf("Could not purge DB: %v", err)
	}
}
