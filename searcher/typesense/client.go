package typesense

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jeremywohl/flatten"
	"github.com/typesense/typesense-go/typesense"
	"github.com/typesense/typesense-go/typesense/api"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Typesense struct {
	client *typesense.Client
}

func NewTypesenseClient(c config.Configuration) (*Typesense, error) {
	client := typesense.NewClient(
		typesense.WithServer(c.Search.Typesense.Host),
		typesense.WithAPIKey(c.Search.Typesense.ApiKey),
	)

	_, err := client.Health(5 * time.Second)
	if err != nil {
		return nil, err
	}

	return &Typesense{client: client}, err
}

func (t *Typesense) Search(collection string, f *datastore.Filter) ([]string, datastore.PaginationData, error) {
	events := make([]string, 0)
	data := datastore.PaginationData{}
	queryByBuilder := new(strings.Builder)
	filterByBuilder := new(strings.Builder)

	filterByBuilder.WriteString(fmt.Sprintf("app_metadata.group_id:=%s", f.Group.UID))

	hasAppFilter := !util.IsStringEmpty(f.AppID)
	if hasAppFilter {
		filterByBuilder.WriteString(fmt.Sprintf(" && app_metadata.uid:=%s", f.Group.UID))
	}

	// CreatedAtEnd and CreatedAtStart are in epoch seconds, but the search records are indexed in milliseconds
	filterByBuilder.WriteString(fmt.Sprintf(" && created_at:[%d..%d]", f.SearchParams.CreatedAtStart*1000, f.SearchParams.CreatedAtEnd*1000))

	col, err := t.client.Collection(collection).Retrieve()
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

	result, err := t.client.Collection(collection).Documents().Search(params)
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

func (t *Typesense) Index(collection string, document convoy.GenericMap) error {
	// convert data field to map
	strData := document["data"].(primitive.Binary).Data
	var data *convoy.GenericMap
	err := json.Unmarshal(strData, &data)
	if err != nil {
		return err
	}

	document["data"] = data
	document["id"] = document["_id"]
	document["updated_at"] = document["updated_at"].(primitive.DateTime).Time().Unix() * 1000
	document["created_at"] = document["created_at"].(primitive.DateTime).Time().Unix() * 1000

	jsonDoc, err := json.Marshal(document)
	if err != nil {
		return err
	}

	flattened, err := flatten.FlattenString(string(jsonDoc), "", flatten.DotStyle)
	if err != nil {
		return err
	}

	var doc *convoy.GenericMap
	err = json.Unmarshal([]byte(flattened), &doc)
	if err != nil {
		return err
	}

	var col *api.CollectionResponse
	collections, err := t.client.Collections().Retrieve()
	if err != nil {
		return err
	}

	for _, c := range collections {
		if c.Name == collection {
			col = c
		}
	}

	if col == nil {
		schema := &api.CollectionSchema{
			Name: collection,
			Fields: []api.Field{
				{Name: ".*", Type: "auto"},
			},
		}

		_, err = t.client.Collections().Create(schema)
		if err != nil {
			return err
		}
	}

	// import to typesense
	_, err = t.client.Collection(collection).Documents().Upsert(doc)
	if err != nil {
		return err
	}

	return nil
}
