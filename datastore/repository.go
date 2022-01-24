package datastore

import (
	"context"
)

type APIKeyRepository interface {
	CreateAPIKey(context.Context, *APIKey) error
	UpdateAPIKey(context.Context, *APIKey) error
	FindAPIKeyByID(context.Context, string) (*APIKey, error)
	FindAPIKeyByMaskID(context.Context, string) (*APIKey, error)
	FindAPIKeyByHash(context.Context, string) (*APIKey, error)
	RevokeAPIKeys(context.Context, []string) error
	LoadAPIKeysPaged(context.Context, *Pageable) ([]APIKey, PaginationData, error)
}

type EventDeliveryRepository interface {
	CreateEventDelivery(context.Context, *EventDelivery) error
	FindEventDeliveryByID(context.Context, string) (*EventDelivery, error)
	FindEventDeliveriesByIDs(context.Context, []string) ([]EventDelivery, error)
	FindEventDeliveriesByEventID(context.Context, string) ([]EventDelivery, error)
	UpdateStatusOfEventDelivery(context.Context, EventDelivery, EventDeliveryStatus) error
	UpdateEventDeliveryWithAttempt(context.Context, EventDelivery, DeliveryAttempt) error
	LoadEventDeliveriesPaged(context.Context, string, string, string, []EventDeliveryStatus, SearchParams, Pageable) ([]EventDelivery, PaginationData, error)
}

type EventRepository interface {
	CreateEvent(context.Context, *Event) error
	LoadEventIntervals(context.Context, string, SearchParams, Period, int) ([]EventInterval, error)
	LoadEventsPagedByAppId(context.Context, string, SearchParams, Pageable) ([]Event, PaginationData, error)
	FindEventByID(ctx context.Context, id string) (*Event, error)
	CountGroupMessages(ctx context.Context, groupID string) (int64, error)
	LoadEventsPaged(context.Context, string, string, SearchParams, Pageable) ([]Event, PaginationData, error)
	DeleteGroupEvents(context.Context, string) error
}

type GroupRepository interface {
	LoadGroups(context.Context, *GroupFilter) ([]*Group, error)
	CreateGroup(context.Context, *Group) error
	UpdateGroup(context.Context, *Group) error
	DeleteGroup(ctx context.Context, uid string) error
	FetchGroupByID(context.Context, string) (*Group, error)
	FetchGroupsByIDs(context.Context, []string) ([]Group, error)
}

type ApplicationRepository interface {
	CreateApplication(context.Context, *Application) error
	LoadApplicationsPaged(context.Context, string, string, Pageable) ([]Application, PaginationData, error)
	FindApplicationByID(context.Context, string) (*Application, error)
	UpdateApplication(context.Context, *Application) error
	DeleteApplication(context.Context, *Application) error
	CountGroupApplications(ctx context.Context, groupID string) (int64, error)
	DeleteGroupApps(context.Context, string) error
	LoadApplicationsPagedByGroupId(context.Context, string, Pageable) ([]Application, PaginationData, error)
	SearchApplicationsByGroupId(context.Context, string, SearchParams) ([]Application, error)
	FindApplicationEndpointByID(context.Context, string, string) (*Endpoint, error)
	UpdateApplicationEndpointsStatus(context.Context, string, []string, EndpointStatus) error
}
