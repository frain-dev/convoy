package noopsearcher

import "github.com/frain-dev/convoy/datastore"

type NoopSearcher struct {
}

func NewNoopSearcher() *NoopSearcher {
	return &NoopSearcher{}
}

func (n *NoopSearcher) Search(filter *datastore.Filter) ([]string, datastore.PaginationData, error) {
	return make([]string, 0), datastore.PaginationData{}, nil
}
