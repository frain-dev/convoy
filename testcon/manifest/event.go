package manifest

import (
	"fmt"
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

func IncEvent(k string) {
	evLock.Lock()
	defer evLock.Unlock()
	count := events[k]
	count++
	events[k] = count
}

func PrintEvents() {
	evLock.RLock()
	defer evLock.RUnlock()
	fmt.Printf("Size: %d Events: %+v\n", len(events), events)
}
