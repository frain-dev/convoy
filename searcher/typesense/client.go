package typesense

import (
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/typesense/typesense-go/typesense"
	"github.com/typesense/typesense-go/typesense/api"
)

type Typesense struct {
	client *typesense.Client
}

func NewTypesenseClient(searchConfig config.SearchConfiguration) (*Typesense, error) {
	client := typesense.NewClient(
		typesense.WithServer(searchConfig.Typesense.Host),
		typesense.WithAPIKey(searchConfig.Typesense.ApiKey),
	)

	_, err := client.Health(5 * time.Second)
	if err != nil {
		return nil, err
	}

	return &Typesense{client: client}, err
}

func (t *Typesense) Search(groupId, query string, pageable datastore.Pageable) ([]string, datastore.PaginationData, error) {
	events := make([]string, 0)
	data := datastore.PaginationData{}
	filter := fmt.Sprintf("app_metadata.group_id:=%s", groupId)

	queryBuilder := new(strings.Builder)

	col, err := t.client.Collection("events").Retrieve()
	if err != nil {
		return events, data, err
	}

	// we can only search string fields for now
	for _, field := range col.Fields {
		if field.Type != "string" {
			continue
		}

		queryBuilder.WriteString(field.Name + ",")
	}

	params := &api.SearchCollectionParams{
		QueryBy:  queryBuilder.String(),
		FilterBy: &filter,
		Page:     &pageable.Page,
		PerPage:  &pageable.PerPage,
		Q:        query,
	}
	result, err := t.client.Collection("events").Documents().Search(params)
	if err != nil {
		return events, data, err
	}

	for _, hit := range *result.Hits {
		events = append(events, (*hit.Document)["uid"].(string))
	}

	data.Next = int64(*result.Page + 1)
	data.Prev = int64(*result.Page - 1)
	data.Page = int64(*result.Page)
	data.Total = int64(*result.OutOf)
	data.TotalPage = int64(*result.OutOf / *result.Found)
	data.PerPage = int64(result.RequestParams.PerPage)

	return events, data, nil
}
