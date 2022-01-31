package queue

import (
	"context"
	"sync"

	"github.com/hashicorp/golang-lru/simplelru"
)

type Storage interface {
	Exists(ctx context.Context, key string) bool
}

var _ Storage = (*localStorage)(nil)

const defaultStorageSize = 128000

// LOCAL

type localStorage struct {
	mu    sync.Mutex
	cache *simplelru.LRU
	size  int
}

func NewLocalStorage() Storage {
	return &localStorage{size: defaultStorageSize}
}

func (s *localStorage) Exists(_ context.Context, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache == nil {
		var err error
		s.cache, err = simplelru.NewLRU(s.size, nil)
		if err != nil {
			panic(err)
		}
	}

	return s.cache.Contains(key)
}
