package manifest

import (
	"sync"
	"sync/atomic"
)

var ctrLock = sync.RWMutex{}

func DecrementAndGet(ctr *atomic.Int64) int64 {
	ctrLock.Lock()
	defer ctrLock.Unlock()
	ctr.Add(-1)
	return ctr.Load()
}
