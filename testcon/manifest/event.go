package manifest

import (
	"sync"
)

var events = map[string]int{}
var evLock = sync.RWMutex{}

func ReadEvent(k string) int {
	evLock.RLock()
	defer evLock.RUnlock()
	return events[k]
}

func WriteEvent(k string, v int) {
	evLock.Lock()
	defer evLock.Unlock()
	events[k] = v
}
