package datastore

import (
	"context"

	"github.com/frain-dev/convoy/server/models"
)

type EventDeliveryRepository interface {
	CreateEventDelivery(context.Context, *EventDelivery) error
	FindEventDeliveryByID(context.Context, string) (*EventDelivery, error)
	FindEventDeliveriesByIDs(context.Context, []string) ([]EventDelivery, error)
	FindEventDeliveriesByEventID(context.Context, string) ([]EventDelivery, error)
	UpdateStatusOfEventDelivery(context.Context, EventDelivery, EventDeliveryStatus) error
	UpdateEventDeliveryWithAttempt(context.Context, EventDelivery, DeliveryAttempt) error
	LoadEventDeliveriesPaged(context.Context, string, string, string, []EventDeliveryStatus, models.SearchParams, models.Pageable) ([]EventDelivery, models.PaginationData, error)
}

type EventRepository interface {
	CreateEvent(context.Context, *Event) error
	LoadEventIntervals(context.Context, string, models.SearchParams, Period, int) ([]models.EventInterval, error)
	LoadEventsPagedByAppId(context.Context, string, models.SearchParams, models.Pageable) ([]Event, models.PaginationData, error)
	FindEventByID(ctx context.Context, id string) (*Event, error)
	CountGroupMessages(ctx context.Context, groupID string) (int64, error)
	LoadEventsScheduledForPosting(context.Context) ([]Event, error)
	LoadEventsForPostingRetry(context.Context) ([]Event, error)
	LoadAbandonedEventsForPostingRetry(context.Context) ([]Event, error)
	LoadEventsPaged(context.Context, string, string, models.SearchParams, models.Pageable) ([]Event, models.PaginationData, error)
	DeleteGroupEvents(context.Context, string) error
}

type GroupRepository interface {
	LoadGroups(context.Context, *GroupFilter) ([]*Group, error)
	CreateGroup(context.Context, *Group) error
	UpdateGroup(context.Context, *Group) error
	DeleteGroup(ctx context.Context, uid string) error
	FetchGroupByID(context.Context, string) (*Group, error)
}

type ApplicationRepository interface {
	CreateApplication(context.Context, *Application) error
	LoadApplicationsPaged(context.Context, string, models.Pageable) ([]Application, models.PaginationData, error)
	FindApplicationByID(context.Context, string) (*Application, error)
	UpdateApplication(context.Context, *Application) error
	DeleteApplication(context.Context, *Application) error
	CountGroupApplications(ctx context.Context, groupID string) (int64, error)
	DeleteGroupApps(context.Context, string) error
	LoadApplicationsPagedByGroupId(context.Context, string, models.Pageable) ([]Application, models.PaginationData, error)
	SearchApplicationsByGroupId(context.Context, string, models.SearchParams) ([]Application, error)
	FindApplicationEndpointByID(context.Context, string, string) (*Endpoint, error)
	UpdateApplicationEndpointsStatus(context.Context, string, []string, EndpointStatus) error
}
