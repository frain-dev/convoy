package testdb

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	ncache "github.com/frain-dev/convoy/cache/noop"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/pkg/httpheader"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
	"github.com/xdg-go/pbkdf2"
)

// SeedEndpoint creates a random endpoint for integration tests.
func SeedEndpoint(db database.Database, g *datastore.Project, uid, title, ownerID string, disabled bool, status datastore.EndpointStatus) (*datastore.Endpoint, error) {
	if util.IsStringEmpty(uid) {
		uid = ulid.Make().String()
	}

	if util.IsStringEmpty(title) {
		title = fmt.Sprintf("TestEndpoint-%s", uid)
	}

	if util.IsStringEmpty(ownerID) {
		ownerID = ulid.Make().String()
	}

	endpoint := &datastore.Endpoint{
		UID:       uid,
		Name:      title,
		ProjectID: g.UID,
		OwnerID:   ownerID,
		Status:    status,
		Secrets:   datastore.Secrets{},
		AppID:     uid,
	}

	// Seed Data.
	endpointRepo := postgres.NewEndpointRepo(db, nil)
	err := endpointRepo.CreateEndpoint(context.TODO(), endpoint, g.UID)
	if err != nil {
		return &datastore.Endpoint{}, err
	}

	return endpoint, nil
}

func SeedMultipleEndpoints(db database.Database, project *datastore.Project, count int) error {
	for i := 0; i < count; i++ {
		uid := ulid.Make().String()
		app := &datastore.Endpoint{
			UID:       uid,
			Name:      fmt.Sprintf("Test-%s", uid),
			ProjectID: project.UID,
			Secrets: datastore.Secrets{
				{UID: ulid.Make().String()},
			},
			AppID: ulid.Make().String(),
		}

		// Seed Data.
		appRepo := postgres.NewEndpointRepo(db, nil)
		err := appRepo.CreateEndpoint(context.TODO(), app, app.ProjectID)
		if err != nil {
			return err
		}
	}
	return nil
}

func SeedEndpointSecret(db database.Database, e *datastore.Endpoint, value string) (*datastore.Secret, error) {
	sc := datastore.Secret{
		UID:   ulid.Make().String(),
		Value: value,
	}

	e.Secrets = append(e.Secrets, sc)

	// Seed Data.
	endpointRepo := postgres.NewEndpointRepo(db, nil)
	err := endpointRepo.UpdateEndpoint(context.TODO(), e, e.ProjectID)
	if err != nil {
		return nil, err
	}

	return &sc, nil
}

func SeedDefaultProject(db database.Database, orgID string) (*datastore.Project, error) {
	if orgID == "" {
		orgID = ulid.Make().String()
	}

	defaultProject := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "default-project",
		Type:           datastore.OutgoingProject,
		OrganisationID: orgID,
		Config: &datastore.ProjectConfig{
			Strategy: &datastore.StrategyConfiguration{
				Type:       datastore.LinearStrategyProvider,
				Duration:   10,
				RetryCount: 2,
			},
			Signature: &datastore.SignatureConfiguration{
				Header: config.DefaultSignatureHeader,
				Versions: []datastore.SignatureVersion{
					{
						UID:       ulid.Make().String(),
						Hash:      "SHA256",
						Encoding:  datastore.HexEncoding,
						CreatedAt: time.Now(),
					},
				},
			},
			ReplayAttacks: false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Seed Data.
	projectRepo := postgres.NewProjectRepo(db, nil)
	err := projectRepo.CreateProject(context.TODO(), defaultProject)
	if err != nil {
		return &datastore.Project{}, err
	}

	return defaultProject, nil
}

const DefaultUserPassword = "password"

// seed default user
func SeedDefaultUser(db database.Database) (*datastore.User, error) {
	p := datastore.Password{Plaintext: DefaultUserPassword}
	err := p.GenerateHash()
	if err != nil {
		return nil, err
	}

	defaultUser := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "default",
		LastName:  "default",
		Email:     "default@user.com",
		Password:  string(p.Hash),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Seed Data.
	userRepo := postgres.NewUserRepo(db, nil)
	err = userRepo.CreateUser(context.TODO(), defaultUser)
	if err != nil {
		return &datastore.User{}, err
	}

	return defaultUser, nil
}

// seed default organisation
func SeedDefaultOrganisation(db database.Database, user *datastore.User) (*datastore.Organisation, error) {
	defaultOrg := &datastore.Organisation{
		UID:       ulid.Make().String(),
		OwnerID:   user.UID,
		Name:      "default-org",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Seed Data.
	organisationRepo := postgres.NewOrgRepo(db, nil)
	err := organisationRepo.CreateOrganisation(context.TODO(), defaultOrg)
	if err != nil {
		return &datastore.Organisation{}, err
	}

	member := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: defaultOrg.UID,
		UserID:         user.UID,
		Role:           auth.Role{Type: auth.RoleSuperUser},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	orgMemberRepo := postgres.NewOrgMemberRepo(db, nil)
	err = orgMemberRepo.CreateOrganisationMember(context.TODO(), member)
	if err != nil {
		return nil, err
	}

	return defaultOrg, nil
}

// seed organisation member
func SeedOrganisationMember(db database.Database, org *datastore.Organisation, user *datastore.User, role *auth.Role) (*datastore.OrganisationMember, error) {
	member := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           *role,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	orgMemberRepo := postgres.NewOrgMemberRepo(db, nil)
	err := orgMemberRepo.CreateOrganisationMember(context.TODO(), member)
	if err != nil {
		return nil, err
	}

	return member, nil
}

// seed organisation invite
func SeedOrganisationInvite(db database.Database, org *datastore.Organisation, email string, role *auth.Role, expiry time.Time, status datastore.InviteStatus) (*datastore.OrganisationInvite, error) {
	if expiry == (time.Time{}) {
		expiry = time.Now()
	}

	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		InviteeEmail:   email,
		OrganisationID: org.UID,
		Role:           *role,
		Token:          uniuri.NewLen(64),
		ExpiresAt:      expiry,
		Status:         status,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	orgInviteRepo := postgres.NewOrgInviteRepo(db, nil)
	err := orgInviteRepo.CreateOrganisationInvite(context.TODO(), iv)
	if err != nil {
		return nil, err
	}

	return iv, nil
}

// SeedAPIKey creates random api key for integration tests.
func SeedAPIKey(db database.Database, role auth.Role, uid, name, keyType, userID string) (*datastore.APIKey, string, error) {
	if util.IsStringEmpty(uid) {
		uid = ulid.Make().String()
	}

	maskID, key := util.GenerateAPIKey()
	salt, err := util.GenerateSecret()
	if err != nil {
		return nil, "", errors.New("failed to generate salt")
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	apiKey := &datastore.APIKey{
		UID:       uid,
		MaskID:    maskID,
		Name:      name,
		UserID:    userID,
		Type:      datastore.KeyType(keyType),
		Role:      role,
		Hash:      encodedKey,
		Salt:      salt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	apiRepo := postgres.NewAPIKeyRepo(db, nil)
	err = apiRepo.CreateAPIKey(context.Background(), apiKey)
	if err != nil {
		return nil, "", err
	}

	return apiKey, key, nil
}

// seed default project
func SeedProject(db database.Database, uid, name, orgID string, projectType datastore.ProjectType, cfg *datastore.ProjectConfig) (*datastore.Project, error) {
	if orgID == "" {
		orgID = ulid.Make().String()
	}
	g := &datastore.Project{
		UID:            uid,
		Name:           name,
		Type:           projectType,
		Config:         cfg,
		OrganisationID: orgID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Seed Data.
	projectRepo := postgres.NewProjectRepo(db, nil)
	err := projectRepo.CreateProject(context.TODO(), g)
	if err != nil {
		return &datastore.Project{}, err
	}

	return g, nil
}

// SeedEvent creates a random event for integration tests.
func SeedEvent(db database.Database, endpoint *datastore.Endpoint, projectID string, uid, eventType string, sourceID string, data []byte) (*datastore.Event, error) {
	if util.IsStringEmpty(uid) {
		uid = ulid.Make().String()
	}

	ev := &datastore.Event{
		UID:       uid,
		EventType: datastore.EventType(eventType),
		Data:      data,
		Endpoints: []string{endpoint.UID},
		Headers:   httpheader.HTTPHeader{},
		ProjectID: projectID,
		SourceID:  sourceID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Seed Data.
	eventRepo := postgres.NewEventRepo(db, nil)
	err := eventRepo.CreateEvent(context.TODO(), ev)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

// SeedEventDelivery creates a random event delivery for integration tests.
func SeedEventDelivery(db database.Database, event *datastore.Event, endpoint *datastore.Endpoint, projectID string, uid string, status datastore.EventDeliveryStatus, subcription *datastore.Subscription) (*datastore.EventDelivery, error) {
	if util.IsStringEmpty(uid) {
		uid = ulid.Make().String()
	}

	eventDelivery := &datastore.EventDelivery{
		UID:            uid,
		EventID:        event.UID,
		EndpointID:     endpoint.UID,
		Status:         status,
		SubscriptionID: subcription.UID,
		Headers:        httpheader.HTTPHeader{},
		Metadata:       &datastore.Metadata{},
		ProjectID:      projectID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Seed Data.
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db, nil)
	err := eventDeliveryRepo.CreateEventDelivery(context.TODO(), eventDelivery)
	if err != nil {
		return nil, err
	}

	return eventDelivery, nil
}

// SeedOrganisation is create random Organisation for integration tests.
func SeedOrganisation(db database.Database, uid, ownerID, name string) (*datastore.Organisation, error) {
	if util.IsStringEmpty(uid) {
		uid = ulid.Make().String()
	}

	if util.IsStringEmpty(name) {
		name = fmt.Sprintf("TestOrg-%s", uid)
	}

	org := &datastore.Organisation{
		UID:       uid,
		OwnerID:   ownerID,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Seed Data.
	orgRepo := postgres.NewOrgRepo(db, nil)
	err := orgRepo.CreateOrganisation(context.TODO(), org)
	if err != nil {
		return &datastore.Organisation{}, err
	}

	return org, nil
}

// SeedMultipleOrganisations creates random Organisations for integration tests.
func SeedMultipleOrganisations(db database.Database, ownerID string, num int) ([]*datastore.Organisation, error) {
	orgs := make([]*datastore.Organisation, 0)

	for i := 0; i < num; i++ {
		uid := ulid.Make().String()

		org := &datastore.Organisation{
			UID:       uid,
			OwnerID:   ownerID,
			Name:      fmt.Sprintf("TestOrg-%s", uid),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		orgs = append(orgs, org)

		// Seed Data.
		orgRepo := postgres.NewOrgRepo(db, nil)
		err := orgRepo.CreateOrganisation(context.TODO(), org)
		if err != nil {
			return nil, err
		}
	}

	return orgs, nil
}

func SeedSource(db database.Database, g *datastore.Project, uid, maskID, ds string, v *datastore.VerifierConfig, customResponseBody, customResponseContentType string) (*datastore.Source, error) {
	if util.IsStringEmpty(uid) {
		uid = ulid.Make().String()
	}

	if util.IsStringEmpty(maskID) {
		maskID = ulid.Make().String()
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
		UID:       uid,
		ProjectID: g.UID,
		MaskID:    maskID,
		Name:      "Convoy-Prod",
		Type:      datastore.SourceType(ds),
		CustomResponse: datastore.CustomResponse{
			Body:        customResponseBody,
			ContentType: customResponseContentType,
		},
		Verifier: v,
	}

	// Seed Data
	sourceRepo := postgres.NewSourceRepo(db, nil)
	err := sourceRepo.CreateSource(context.TODO(), source)
	if err != nil {
		return nil, err
	}

	return source, nil
}

func SeedSubscription(db database.Database,
	g *datastore.Project,
	uid string,
	projectType datastore.ProjectType,
	source *datastore.Source,
	endpoint *datastore.Endpoint,
	retryConfig *datastore.RetryConfiguration,
	alertConfig *datastore.AlertConfiguration,
	filterConfig *datastore.FilterConfiguration,
) (*datastore.Subscription, error) {
	if util.IsStringEmpty(uid) {
		uid = ulid.Make().String()
	}

	if filterConfig == nil {
		filterConfig = &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
			Filter: datastore.FilterSchema{
				Headers: datastore.M{},
				Body:    datastore.M{},
			},
		}
	}

	subscription := &datastore.Subscription{
		UID:        uid,
		ProjectID:  g.UID,
		Name:       "",
		Type:       datastore.SubscriptionTypeAPI,
		SourceID:   source.UID,
		EndpointID: endpoint.UID,

		RetryConfig:  retryConfig,
		AlertConfig:  alertConfig,
		FilterConfig: filterConfig,
	}

	subRepo := postgres.NewSubscriptionRepo(db, nil)
	err := subRepo.CreateSubscription(context.TODO(), g.UID, subscription)
	if err != nil {
		return nil, err
	}

	return subscription, nil
}

func SeedUser(db database.Database, email, password string) (*datastore.User, error) {
	p := &datastore.Password{Plaintext: password}
	err := p.GenerateHash()
	if err != nil {
		return nil, err
	}

	if email == "" {
		email = "test@test.com"
	}

	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "test",
		LastName:  "test",
		Password:  string(p.Hash),
		Email:     email,
	}

	// Seed Data
	userRepo := postgres.NewUserRepo(db, nil)
	err = userRepo.CreateUser(context.TODO(), user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func SeedConfiguration(db database.Database) (*datastore.Configuration, error) {
	config := &datastore.Configuration{
		UID:                ulid.Make().String(),
		IsAnalyticsEnabled: true,
		IsSignupEnabled:    true,
		StoragePolicy:      &datastore.DefaultStoragePolicy,
	}

	// Seed Data
	configRepo := postgres.NewConfigRepo(db)
	err := configRepo.CreateConfiguration(context.TODO(), config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func SeedDevice(db database.Database, g *datastore.Project, endpointID string) error {
	device := &datastore.Device{
		UID:        ulid.Make().String(),
		ProjectID:  g.UID,
		EndpointID: endpointID,
		HostName:   "",
		Status:     datastore.DeviceStatusOnline,
	}

	deviceRepo := postgres.NewDeviceRepo(db, nil)
	err := deviceRepo.CreateDevice(context.TODO(), device)
	if err != nil {
		return err
	}

	return nil
}

func SeedPortalLink(db database.Database, g *datastore.Project, endpoints []string) (*datastore.PortalLink, error) {
	portalLink := &datastore.PortalLink{
		UID:       ulid.Make().String(),
		ProjectID: g.UID,
		Name:      fmt.Sprintf("TestPortalLink-%s", ulid.Make().String()),
		Token:     ulid.Make().String(),
		Endpoints: endpoints,
	}

	portalLinkRepo := postgres.NewPortalLinkRepo(db, nil)
	err := portalLinkRepo.CreatePortalLink(context.TODO(), portalLink)
	if err != nil {
		return nil, err
	}

	return portalLink, nil
}

func SeedMetaEvent(db database.Database, project *datastore.Project) (*datastore.MetaEvent, error) {
	metaEvent := &datastore.MetaEvent{
		UID:       ulid.Make().String(),
		Status:    datastore.ScheduledEventStatus,
		EventType: string(datastore.EndpointCreated),
		ProjectID: project.UID,
		Metadata: &datastore.Metadata{
			Data:            []byte(`{"name": "10x"}`),
			Raw:             `{"name": "10x"}`,
			Strategy:        datastore.ExponentialStrategyProvider,
			NextSendTime:    time.Now().Add(time.Hour),
			NumTrials:       1,
			IntervalSeconds: 10,
			RetryLimit:      20,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	metaEventRepo := postgres.NewMetaEventRepo(db, nil)
	err := metaEventRepo.CreateMetaEvent(context.TODO(), metaEvent)
	if err != nil {
		return nil, err
	}

	return metaEvent, nil
}

func SeedCatalogue(db database.Database, project *datastore.Project, t datastore.CatalogueType, openapiSpec []byte, events datastore.EventDataCatalogues) (*datastore.EventCatalogue, error) {
	c := &datastore.EventCatalogue{
		UID:         ulid.Make().String(),
		ProjectID:   project.UID,
		Type:        t,
		Events:      events,
		OpenAPISpec: openapiSpec,
		CreatedAt:   time.Time{},
		UpdatedAt:   time.Time{},
		DeletedAt:   null.Time{},
	}

	err := postgres.NewEventCatalogueRepo(db, &ncache.NoopCache{}).CreateEventCatalogue(context.TODO(), c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// PurgeDB is run after every test run, and it's used to truncate the DB to have
// a clean slate in the next run.
func PurgeDB(t *testing.T, db database.Database) {
	err := truncateTables(db)
	if err != nil {
		t.Fatalf("Could not purge DB: %v", err)
	}
}

func truncateTables(db database.Database) error {
	tables := `
		convoy.event_deliveries,
		convoy.events,
		convoy.api_keys,
		convoy.subscriptions,
		convoy.source_verifiers,
		convoy.sources,
		convoy.configurations,
		convoy.devices,
		convoy.portal_links,
		convoy.organisation_invites,
		convoy.applications,
        convoy.endpoints,
		convoy.projects,
		convoy.project_configurations,
		convoy.organisation_members,
		convoy.organisations,
		convoy.users,
		convoy.jobs
	`

	_, err := db.GetDB().ExecContext(context.Background(), fmt.Sprintf("TRUNCATE %s CASCADE;", tables))
	if err != nil {
		return err
	}

	return nil
}
