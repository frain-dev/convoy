package datastore

import (
	"context"

	"github.com/frain-dev/convoy"
)

type DatabaseClient interface {
	GetName() string
	Client() interface{}
	Disconnect(context.Context) error

	GroupRepo() convoy.GroupRepository
	EventRepo() convoy.EventRepository
	AppRepo() convoy.ApplicationRepository
	EventDeliveryRepo() convoy.EventDeliveryRepository
}
