package manifest

import (
	"fmt"
	"sync"
)

var endpoints = map[string]int{}
var lock = sync.RWMutex{}

func ReadEndpoint(k string) int {
	lock.RLock()
	defer lock.RUnlock()
	return endpoints[k]
}

func WriteEndpoint(k string, v int) {
	lock.Lock()
	defer lock.Unlock()
	endpoints[k] = v
}

func IncEndpoint(k string) {
	lock.Lock()
	defer lock.Unlock()
	count := endpoints[k]
	count++
	endpoints[k] = count
}

func PrintEndpoints() {
	lock.RLock()
	defer lock.RUnlock()
	fmt.Printf("Size: %d Endpoints: %+v\n", len(endpoints), endpoints)
}
