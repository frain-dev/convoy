package hooks

import (
	"errors"
	"sync/atomic"

	"github.com/frain-dev/convoy/database/listener"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/mevent"
	"github.com/frain-dev/convoy/queue"
)

type hookMap map[datastore.HookEventType]func(data interface{})

type Hook struct {
	fns              hookMap
	endpointListener *listener.EndpointListener
	// eventDeliveryListener datastore.Listener
}

var (
	hookSingleton atomic.Value
)

func Get() (*Hook, error) {
	ho, ok := hookSingleton.Load().(*Hook)
	if !ok {
		return &Hook{}, errors.New("call Init before this function")
	}

	return ho, nil
}

func Init(queue queue.Queuer, projectRepo datastore.ProjectRepository, metaEventRepo datastore.MetaEventRepository) {
	hc := &Hook{fns: hookMap{}}

	metaEvent := mevent.NewMetaEvent(queue, projectRepo, metaEventRepo)
	hc.endpointListener = listener.NewEndpointListener(metaEvent)

	hc.registerHooks()
	hookSingleton.Store(hc)
}

func (h *Hook) Fire(eventType datastore.HookEventType, data interface{}) {
	if fn, ok := h.fns[eventType]; ok {
		fn(data)
	}
}

func (h *Hook) registerHooks() {
	h.fns[datastore.EndpointCreated] = h.endpointListener.AfterCreate
	h.fns[datastore.EndpointUpdated] = h.endpointListener.AfterUpdate
	h.fns[datastore.EndpointDeleted] = h.endpointListener.AfterDelete
}
