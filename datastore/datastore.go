package datastore

import (
	"context"
)

type DatabaseClient interface {
	GetName() string
	Client() interface{}
	Disconnect(context.Context) error

	APIRepo() APIKeyRepository
	GroupRepo() GroupRepository
	EventRepo() EventRepository
	AppRepo() ApplicationRepository
	EventDeliveryRepo() EventDeliveryRepository
	SourceRepo() SourceRepository
}
