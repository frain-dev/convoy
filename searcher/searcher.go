package searcher

import (
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	noopsearcher "github.com/frain-dev/convoy/searcher/noop"
	"github.com/frain-dev/convoy/searcher/typesense"
)

type Searcher interface {
	Search(collection string, filter *datastore.Filter) ([]string, datastore.PaginationData, error)
	Index(collection string, document convoy.GenericMap) error
}

func NewSearchClient(c config.Configuration) (Searcher, error) {
	if c.Search.Type == config.SearchProvider("typesense") {
		if c.Database.Type != "mongodb" {
			return nil, convoy.ErrUnsupportedDatebase
		}

		client, err := typesense.NewTypesenseClient(c)
		return client, err
	}

	return noopsearcher.NewNoopSearcher(), nil
}
