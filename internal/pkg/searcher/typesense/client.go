package typesense

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jeremywohl/flatten"
	"github.com/typesense/typesense-go/typesense"
	"github.com/typesense/typesense-go/typesense/api"
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

	filterByBuilder.WriteString(fmt.Sprintf("group_id:=%s", f.Group.UID))

	hasAppFilter := !util.IsStringEmpty(f.AppID)
	if hasAppFilter {
		filterByBuilder.WriteString(fmt.Sprintf(" && app_id:=%s", f.Group.UID))
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

func (t *Typesense) Index(collection string, rawDocument interface{}) error {
	event := rawDocument.(datastore.Event)

	// convert event data field to map
	rawData := event.Data
	var eventData *convoy.GenericMap
	err := json.Unmarshal(rawData, &eventData)
	if err != nil {
		return err
	}

	// convert event to map
	eBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	var document convoy.GenericMap
	err = json.Unmarshal(eBytes, &document)
	if err != nil {
		return err
	}

	document["data"] = eventData
	document["id"] = document["uid"]

	createdAt, err := time.Parse("2006-01-02T15:04:05Z07:00", document["created_at"].(string))
	if err != nil {
		return err
	}
	document["created_at"] = createdAt.Unix()

	updatedAt, err := time.Parse("2006-01-02T15:04:05Z07:00", document["updated_at"].(string))
	if err != nil {
		return err
	}
	document["updated_at"] = updatedAt.Unix()

	jsonDoc, err := json.Marshal(document)
	if err != nil {
		return err
	}

	flattened, err := flatten.FlattenString(string(jsonDoc), "", flatten.DotStyle)
	if err != nil {
		return err
	}

	var indexedDoc *convoy.GenericMap
	err = json.Unmarshal([]byte(flattened), &indexedDoc)
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
	_, err = t.client.Collection(collection).Documents().Upsert(indexedDoc)
	if err != nil {
		return err
	}

	return nil
}

func (t *Typesense) Remove(collection string, f *datastore.Filter) error {
	filterByBuilder := new(strings.Builder)

	filterByBuilder.WriteString(fmt.Sprintf("group_id:=%s", f.Group.UID))

	// CreatedAtEnd and CreatedAtStart are in epoch seconds, but the search records are indexed in milliseconds
	filterByBuilder.WriteString(fmt.Sprintf(" && created_at:[%d..%d]", f.SearchParams.CreatedAtStart*1000, f.SearchParams.CreatedAtEnd*1000))

	filterByBuilder.WriteString(fmt.Sprintf(" && created_at:[%d..%d]", f.SearchParams.CreatedAtStart*1000, f.SearchParams.CreatedAtEnd*1000))
	filterBy := filterByBuilder.String()
	batchsize := 100

	filter := &api.DeleteDocumentsParams{
		FilterBy:  &filterBy,
		BatchSize: &batchsize}
	c, err := t.client.Collection(collection).Documents().Delete(filter)
	if err != nil {
		return err
	}
	log.Printf("Num of docs deleted %d", c)
	return nil
}
