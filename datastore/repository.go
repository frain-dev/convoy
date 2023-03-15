package datastore

import (
	"context"
	"encoding/json"
	"time"
)

type APIKeyRepository interface {
	CreateAPIKey(context.Context, *APIKey) error
	UpdateAPIKey(context.Context, *APIKey) error
	FindAPIKeyByID(context.Context, string) (*APIKey, error)
	FindAPIKeyByProjectID(context.Context, string) (*APIKey, error)
	FindAPIKeyByMaskID(context.Context, string) (*APIKey, error)
	FindAPIKeyByHash(context.Context, string) (*APIKey, error)
	RevokeAPIKeys(context.Context, []string) error
	LoadAPIKeysPaged(context.Context, *ApiKeyFilter, *Pageable) ([]APIKey, PaginationData, error)
}

type EventDeliveryRepository interface {
	CreateEventDelivery(context.Context, *EventDelivery) error
	FindEventDeliveryByID(ctx context.Context, projectID string, id string) (*EventDelivery, error)
	FindEventDeliveriesByIDs(ctx context.Context, projectID string, ids []string) ([]EventDelivery, error)
	FindEventDeliveriesByEventID(ctx context.Context, projectID string, id string) ([]EventDelivery, error)
	CountDeliveriesByStatus(ctx context.Context, projectID string, status EventDeliveryStatus, params SearchParams) (int64, error)
	UpdateStatusOfEventDelivery(ctx context.Context, projectID string, eventDelivery EventDelivery, status EventDeliveryStatus) error
	UpdateStatusOfEventDeliveries(ctx context.Context, projectID string, ids []string, status EventDeliveryStatus) error
	FindDiscardedEventDeliveries(ctx context.Context, projectID, endpointId, deviceId string, params SearchParams) ([]EventDelivery, error)

	UpdateEventDeliveryWithAttempt(ctx context.Context, projectID string, eventDelivery EventDelivery, attempt DeliveryAttempt) error
	CountEventDeliveries(ctx context.Context, projectID string, endpointIDs []string, eventID string, status []EventDeliveryStatus, params SearchParams) (int64, error)
	DeleteProjectEventDeliveries(ctx context.Context, projectID string, filter *EventDeliveryFilter, hardDelete bool) error
	LoadEventDeliveriesPaged(ctx context.Context, projectID string, endpointIDs []string, eventID string, status []EventDeliveryStatus, params SearchParams, pageable Pageable) ([]EventDelivery, PaginationData, error)
	LoadEventDeliveriesIntervals(ctx context.Context, projectID string, params SearchParams, period Period, interval int) ([]EventInterval, error)
}

type EventRepository interface {
	CreateEvent(context.Context, *Event) error
	FindEventByID(ctx context.Context, projectID string, id string) (*Event, error)
	FindEventsByIDs(ctx context.Context, projectID string, ids []string) ([]Event, error)
	CountProjectMessages(ctx context.Context, projectID string) (int64, error)
	CountEvents(ctx context.Context, projectID string, f *Filter) (int64, error)
	LoadEventsPaged(ctx context.Context, projectID string, f *Filter) ([]Event, PaginationData, error)
	DeleteProjectEvents(ctx context.Context, projectID string, f *EventFilter, hardDelete bool) error
}

type ProjectRepository interface {
	LoadProjects(context.Context, *ProjectFilter) ([]*Project, error)
	CreateProject(context.Context, *Project) error
	UpdateProject(context.Context, *Project) error
	DeleteProject(ctx context.Context, uid string) error
	FetchProjectByID(context.Context, string) (*Project, error)
	FillProjectsStatistics(ctx context.Context, project *Project) error
}

type OrganisationRepository interface {
	LoadOrganisationsPaged(context.Context, Pageable) ([]Organisation, PaginationData, error)
	CreateOrganisation(context.Context, *Organisation) error
	UpdateOrganisation(context.Context, *Organisation) error
	DeleteOrganisation(context.Context, string) error
	FetchOrganisationByID(context.Context, string) (*Organisation, error)
	FetchOrganisationByCustomDomain(context.Context, string) (*Organisation, error)
	FetchOrganisationByAssignedDomain(context.Context, string) (*Organisation, error)
}

type OrganisationInviteRepository interface {
	LoadOrganisationsInvitesPaged(ctx context.Context, orgID string, inviteStatus InviteStatus, pageable Pageable) ([]OrganisationInvite, PaginationData, error)
	CreateOrganisationInvite(ctx context.Context, iv *OrganisationInvite) error
	UpdateOrganisationInvite(ctx context.Context, iv *OrganisationInvite) error
	DeleteOrganisationInvite(ctx context.Context, uid string) error
	FetchOrganisationInviteByID(ctx context.Context, uid string) (*OrganisationInvite, error)
	FetchOrganisationInviteByToken(ctx context.Context, token string) (*OrganisationInvite, error)
}

type OrganisationMemberRepository interface {
	LoadOrganisationMembersPaged(ctx context.Context, organisationID string, pageable Pageable) ([]*OrganisationMember, PaginationData, error)
	LoadUserOrganisationsPaged(ctx context.Context, userID string, pageable Pageable) ([]Organisation, PaginationData, error)
	FindUserProjects(ctx context.Context, userID string) ([]Project, error)
	CreateOrganisationMember(ctx context.Context, member *OrganisationMember) error
	UpdateOrganisationMember(ctx context.Context, member *OrganisationMember) error
	DeleteOrganisationMember(ctx context.Context, memberID string, orgID string) error
	FetchOrganisationMemberByID(ctx context.Context, memberID string, organisationID string) (*OrganisationMember, error)
	FetchOrganisationMemberByUserID(ctx context.Context, userID string, organisationID string) (*OrganisationMember, error)
}

type EndpointRepository interface {
	CreateEndpoint(ctx context.Context, endpoint *Endpoint, projectID string) error
	FindEndpointByID(çtx context.Context, id string, projectID string) (*Endpoint, error)
	FindEndpointsByID(ctx context.Context, ids []string, projectID string) ([]Endpoint, error)
	FindEndpointsByAppID(ctx context.Context, appID string, projectID string) ([]Endpoint, error)
	FindEndpointsByOwnerID(ctx context.Context, projectID string, ownerID string) ([]Endpoint, error)
	UpdateEndpoint(ctx context.Context, endpoint *Endpoint, projectID string) error
	UpdateEndpointStatus(ctx context.Context, projectID, endpointID string, status EndpointStatus) error
	DeleteEndpoint(ctx context.Context, endpoint *Endpoint, projectID string) error
	CountProjectEndpoints(ctx context.Context, projectID string) (int64, error)
	LoadEndpointsPaged(ctx context.Context, projectID string, query string, pageable Pageable) ([]Endpoint, PaginationData, error)
	UpdateSecrets(ctx context.Context, endpointID string, projectID string, secrets Secrets) error
	DeleteSecret(ctx context.Context, endpoint *Endpoint, secretID string, projectID string) error
}
type SubscriptionRepository interface {
	CreateSubscription(context.Context, string, *Subscription) error
	UpdateSubscription(ctx context.Context, projectID string, subscription *Subscription) error
	LoadSubscriptionsPaged(ctx context.Context, projectID string, filter *FilterBy, pageable Pageable) ([]Subscription, PaginationData, error)
	DeleteSubscription(ctx context.Context, projectID string, subscription *Subscription) error
	FindSubscriptionByID(ctx context.Context, projectID, id string) (*Subscription, error)
	FindSubscriptionsBySourceID(ctx context.Context, projectID, sourceID string) ([]Subscription, error)
	FindSubscriptionsByEndpointID(ctx context.Context, projectId string, endpointID string) ([]Subscription, error)
	FindSubscriptionByDeviceID(ctx context.Context, projectId string, deviceID string, subscriptionType SubscriptionType) (*Subscription, error)
	FindCLISubscriptions(ctx context.Context, projectID string) ([]Subscription, error)
	TestSubscriptionFilter(ctx context.Context, payload map[string]interface{}, filter map[string]interface{}) (bool, error)
}

type SourceRepository interface {
	CreateSource(context.Context, *Source) error
	UpdateSource(ctx context.Context, projectID string, source *Source) error
	FindSourceByID(ctx context.Context, projectID string, id string) (*Source, error)
	FindSourceByName(ctx context.Context, projectId string, name string) (*Source, error)
	FindSourceByMaskID(ctx context.Context, maskID string) (*Source, error)
	DeleteSourceByID(ctx context.Context, projectID string, id string, sourceVerifierID string) error
	LoadSourcesPaged(ctx context.Context, projectID string, filter *SourceFilter, pageable Pageable) ([]Source, PaginationData, error)
}

type DeviceRepository interface {
	CreateDevice(ctx context.Context, device *Device) error
	UpdateDevice(ctx context.Context, device *Device, appID, projectID string) error
	UpdateDeviceLastSeen(ctx context.Context, device *Device, appID, projectID string, status DeviceStatus) error
	DeleteDevice(ctx context.Context, uid string, appID, projectID string) error
	FetchDeviceByID(ctx context.Context, uid string, appID, projectID string) (*Device, error)
	FetchDeviceByHostName(ctx context.Context, hostName string, appID, projectID string) (*Device, error)
	LoadDevicesPaged(ctx context.Context, projectID string, filter *ApiKeyFilter, pageable Pageable) ([]Device, PaginationData, error)
}

type UserRepository interface {
	CreateUser(context.Context, *User) error
	UpdateUser(ctx context.Context, user *User) error
	FindUserByEmail(context.Context, string) (*User, error)
	FindUserByID(context.Context, string) (*User, error)
	FindUserByToken(context.Context, string) (*User, error)
	FindUserByEmailVerificationToken(ctx context.Context, token string) (*User, error)
	LoadUsersPaged(context.Context, Pageable) ([]User, PaginationData, error)
}

type ConfigurationRepository interface {
	CreateConfiguration(context.Context, *Configuration) error
	LoadConfiguration(context.Context) (*Configuration, error)
	UpdateConfiguration(context.Context, *Configuration) error
}

type PortalLinkRepository interface {
	CreatePortalLink(context.Context, *PortalLink) error
	UpdatePortalLink(ctx context.Context, projectID string, portal *PortalLink) error
	FindPortalLinkByID(ctx context.Context, projectID string, id string) (*PortalLink, error)
	FindPortalLinkByToken(ctx context.Context, token string) (*PortalLink, error)
	LoadPortalLinksPaged(ctx context.Context, projectID string, f *FilterBy, pageable Pageable) ([]PortalLink, PaginationData, error)
	RevokePortalLink(ctx context.Context, projectID string, id string) error
}

type ExportRepository interface {
	ExportRecords(ctx context.Context, tableName, projectID string, createdAt time.Time) (json.RawMessage, int64, error)
}
