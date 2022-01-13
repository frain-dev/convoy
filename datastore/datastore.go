package datastore

import (
	"context"
)

type DatabaseClient interface {
	GetName() string
	Client() interface{}
	Disconnect(context.Context) error

	GroupRepo() GroupRepository
	EventRepo() EventRepository
	AppRepo() ApplicationRepository
	EventDeliveryRepo() EventDeliveryRepository
}
