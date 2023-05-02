package hooks

import (
	"errors"
	"sync/atomic"

	"github.com/frain-dev/convoy/datastore"
)

type hookMap map[datastore.HookEventType]func(data interface{})

type Hook struct {
	fns hookMap
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

func Init() *Hook {
	return &Hook{fns: hookMap{}}
}

func (h *Hook) Fire(eventType datastore.HookEventType, data interface{}) {
	if fn, ok := h.fns[eventType]; ok {
		fn(data)
	}
}

func (h *Hook) RegisterHook(eventType datastore.HookEventType, fn func(data interface{})) {
	h.fns[eventType] = fn
	hookSingleton.Store(h)
}
