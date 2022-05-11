package typesense

import (
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
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

func (t *Typesense) Search(f *datastore.Filter) ([]string, datastore.PaginationData, error) {
	events := make([]string, 0)
	data := datastore.PaginationData{}
	queryByBuilder := new(strings.Builder)
	filterByBuilder := new(strings.Builder)

	filterByBuilder.WriteString(fmt.Sprintf("app_metadata.group_id:=%s", f.Group.UID))

	hasAppFilter := !util.IsStringEmpty(f.AppID)
	if hasAppFilter {
		filterByBuilder.WriteString(fmt.Sprintf(" && app_metadata.uid:=%s", f.Group.UID))
	}

	filterByBuilder.WriteString(fmt.Sprintf(" && created_at:[%d..%d]", f.SearchParams.CreatedAtStart*1000, f.SearchParams.CreatedAtEnd*1000))

	col, err := t.client.Collection("events").Retrieve()
	if err != nil {
		return events, data, err
	}

	// we can only search string fields for now
	for _, field := range col.Fields {
		if field.Type != "string" {
			continue
		}

		queryByBuilder.WriteString(field.Name + ",")
	}

	sortBy := "created_at:desc"
	queryBy := queryByBuilder.String()
	filterBy := filterByBuilder.String()

	params := &api.SearchCollectionParams{
		Q:        f.Query,
		QueryBy:  queryBy,
		SortBy:   &sortBy,
		FilterBy: &filterBy,
		Page:     &f.Pageable.Page,
		PerPage:  &f.Pageable.PerPage,
	}

	result, err := t.client.Collection("events").Documents().Search(params)
	if err != nil {
		return events, data, err
	}

	for _, hit := range *result.Hits {
		events = append(events, (*hit.Document)["uid"].(string))
	}

	data.Next = int64(f.Pageable.Page + 1)
	data.Prev = int64(f.Pageable.Page - 1)
	data.Page = int64(f.Pageable.Page)
	data.Total = int64(*result.OutOf)
	data.PerPage = int64(f.Pageable.PerPage)

	if *result.Found > 0 {
		data.TotalPage = int64(*result.Found / f.Pageable.PerPage)
	} else {
		data.TotalPage = 0
	}

	return events, data, nil
}
