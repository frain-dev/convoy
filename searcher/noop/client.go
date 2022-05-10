package noopsearcher

import "github.com/frain-dev/convoy/datastore"

type NoopSearcher struct {
}

func NewNoopSearcher() *NoopSearcher {
	return &NoopSearcher{}
}

func (n *NoopSearcher) Search(groupId, query string, pageable datastore.Pageable) ([]string, datastore.PaginationData, error) {
	return make([]string, 0), datastore.PaginationData{}, nil
}
