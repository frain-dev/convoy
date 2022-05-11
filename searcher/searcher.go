package searcher

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	noopsearcher "github.com/frain-dev/convoy/searcher/noop"
	"github.com/frain-dev/convoy/searcher/typesense"
)

type Searcher interface {
	Search(filter *datastore.Filter) ([]string, datastore.PaginationData, error)
}

func NewSearchClient(searchConfig config.SearchConfiguration) (Searcher, error) {
	if searchConfig.Type == config.SearchProvider("typesense") {
		client, err := typesense.NewTypesenseClient(searchConfig)
		return client, err
	}

	return noopsearcher.NewNoopSearcher(), nil
}
