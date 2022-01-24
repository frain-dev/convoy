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

// LOCAL

type localStorage struct {
	mu    sync.Mutex
	cache *simplelru.LRU
}

func NewLocalStorage() Storage {
	return &localStorage{}
}

func (s *localStorage) Exists(_ context.Context, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache == nil {
		var err error
		s.cache, err = simplelru.NewLRU(128000, nil)
		if err != nil {
			panic(err)
		}
	}

	_, ok := s.cache.Get(key)
	if ok {
		return true
	}

	s.cache.Add(key, nil)
	return false
}
