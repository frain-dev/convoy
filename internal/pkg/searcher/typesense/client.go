package typesense

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jeremywohl/flatten"
	"github.com/typesense/typesense-go/typesense"
	"github.com/typesense/typesense-go/typesense/api"
)

const DateFormat = "2006-01-02T15:04:05Z07:00"

var ErrIDFieldIsRequired = errors.New("id field does not exist on the document")
var ErrCreatedAtFieldIsRequired = errors.New("created_at field should be a string")
var ErrCreatedAtFieldIsNotString = errors.New("created_at field does not exist on the document")

var ErrUpdatedAtFieldIsRequired = errors.New("updated_at field should be a string")
var ErrUpdatedAtFieldIsNotString = errors.New("updated_at field does not exist on the document")

type Typesense struct {
	client *typesense.Client
}

func NewTypesenseClient(host, apiKey string) (*Typesense, error) {
	client := typesense.NewClient(typesense.WithServer(host), typesense.WithAPIKey(apiKey))

	_, err := client.Health(5 * time.Second)
	if err != nil {
		return nil, err
	}

	return &Typesense{client: client}, err
}

func (t *Typesense) Search(collection string, f *datastore.SearchFilter) ([]convoy.GenericMap, datastore.PaginationData, error) {
	docs := make([]convoy.GenericMap, 0)
	data := datastore.PaginationData{}
	queryByBuilder := new(strings.Builder)

	col, err := t.client.Collection(collection).Retrieve()
	if err != nil {
		return docs, data, err
	}

	// we can only search string fields for now
	for _, field := range col.Fields {
		if field.Type != "string" {
			continue
		}

		queryByBuilder.WriteString(field.Name + ",")
	}

	queryBy := queryByBuilder.String()
	sortBy := "created_at:desc"

	params := &api.SearchCollectionParams{
		Q:        f.Query,
		QueryBy:  queryBy,
		SortBy:   &sortBy,
		FilterBy: &f.FilterBy,
		Page:     &f.Pageable.Page,
		PerPage:  &f.Pageable.PerPage,
	}

	result, err := t.client.Collection(collection).Documents().Search(params)
	if err != nil {
		return docs, data, err
	}

	for _, hit := range *result.Hits {
		docs = append(docs, *hit.Document)
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

	return docs, data, nil
}

func (t *Typesense) Index(collection string, document convoy.GenericMap) error {
	// perform schema validation
	if _, found := document["id"]; !found {
		return ErrIDFieldIsRequired
	}

	if c, found := document["created_at"]; found {
		if created_at, ok := c.(string); ok {
			createdAt, err := time.Parse(DateFormat, created_at)
			if err != nil {
				return err
			}
			document["created_at"] = createdAt.Unix()
		} else {
			return ErrCreatedAtFieldIsNotString
		}
	} else {
		return ErrCreatedAtFieldIsRequired
	}

	if u, found := document["updated_at"]; found {
		if updated_at, ok := u.(string); ok {
			updatedAt, err := time.Parse(DateFormat, updated_at)
			if err != nil {
				return err
			}
			document["updated_at"] = updatedAt.Unix()
		} else {
			return ErrUpdatedAtFieldIsNotString
		}
	} else {
		return ErrUpdatedAtFieldIsRequired
	}

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

func (t *Typesense) Remove(collection string, f *datastore.SearchFilter) error {
	batchsize := 100
	filter := &api.DeleteDocumentsParams{FilterBy: &f.FilterBy, BatchSize: &batchsize}

	c, err := t.client.Collection(collection).Documents().Delete(filter)
	if err != nil {
		return err
	}

	log.Printf("Deleted %d documents", c)
	return nil
}
