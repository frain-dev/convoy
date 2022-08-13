package noopsearcher

import (
	"github.com/frain-dev/convoy/datastore"
)

type NoopSearcher struct {
}

func NewNoopSearcher() *NoopSearcher {
	return &NoopSearcher{}
}

func (n *NoopSearcher) Search(collection string, filter *datastore.Filter) ([]string, datastore.PaginationData, error) {
	return make([]string, 0), datastore.PaginationData{}, nil
}

func (n *NoopSearcher) Index(collection string, document interface{}) error {
	return nil
}

func (n *NoopSearcher) Remove(collection string, filter *datastore.Filter) error {
	return nil
}
