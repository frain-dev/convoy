package services

import (
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
)

type EndpointService struct {
	projectRepo       datastore.ProjectRepository
	endpointRepo      datastore.EndpointRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	cache             cache.Cache
	queue             queue.Queuer
}

func NewEndpointService(projectRepo datastore.ProjectRepository, endpointRepo datastore.EndpointRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, cache cache.Cache, queue queue.Queuer) *EndpointService {
	return &EndpointService{
		projectRepo:       projectRepo,
		endpointRepo:      endpointRepo,
		eventRepo:         eventRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		cache:             cache,
		queue:             queue,
	}
}
