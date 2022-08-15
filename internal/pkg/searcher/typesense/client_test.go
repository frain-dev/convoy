//go:build integration
// +build integration

package typesense

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

const testCollection = "test"

type Person struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Age       int    `json:"age,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func (p *Person) toGenericMap(document *convoy.GenericMap) error {
	// convert event to map
	eBytes, err := json.Marshal(p)
	if err != nil {
		return err
	}

	err = json.Unmarshal(eBytes, &document)
	if err != nil {
		return err
	}

	return nil
}

func getTypesenseHost() string {
	return "http://localhost:8108"
	// return os.Getenv("CONVOY_TYPESENSE_HOST")
}

func getTypesenseAPIKey() string {
	return "some-api-key"
	// return os.Getenv("CONVOY_TYPESENSE_API_KEY")
}

func deleteCollection(t *testing.T, ts *Typesense, collection string) {
	collections, err := ts.client.Collections().Retrieve()
	require.NoError(t, err)

	for _, c := range collections {
		if c.Name == collection {
			_, err := ts.client.Collection(collection).Delete()
			require.NoError(t, err)
			break
		}
	}
}

func Test_IndexOne(t *testing.T) {
	ts, err := NewTypesenseClient(getTypesenseHost(), getTypesenseAPIKey())
	require.NoError(t, err)
	defer deleteCollection(t, ts, testCollection)

	p := Person{
		Age:       1,
		Name:      "raymond",
		ID:        "uid-1",
		CreatedAt: "2022-08-02T15:04:05+01:00",
		UpdatedAt: "2022-09-02T15:04:05+01:00",
	}

	var doc convoy.GenericMap
	err = p.toGenericMap(&doc)
	require.NoError(t, err)

	err = ts.Index(testCollection, doc)
	require.NoError(t, err)

	col, err := ts.client.Collection(testCollection).Retrieve()
	require.NoError(t, err)

	require.Equal(t, int64(1), col.NumDocuments)
}

func Test_IndexMutiple(t *testing.T) {
	ts, err := NewTypesenseClient(getTypesenseHost(), getTypesenseAPIKey())
	require.NoError(t, err)
	defer deleteCollection(t, ts, testCollection)

	people := []Person{
		{
			Age:       1,
			Name:      "subomi",
			ID:        "uid-1",
			CreatedAt: "2022-09-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			Age:       2,
			Name:      "raymond",
			ID:        "uid-2",
			CreatedAt: "2022-08-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			Age:       2,
			Name:      "emmanuel",
			ID:        "uid-3",
			CreatedAt: "2022-08-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
	}

	for _, p := range people {
		var doc convoy.GenericMap
		err = p.toGenericMap(&doc)
		require.NoError(t, err)

		err = ts.Index(testCollection, doc)
		require.NoError(t, err)
	}

	col, err := ts.client.Collection(testCollection).Retrieve()
	require.NoError(t, err)

	require.Equal(t, int64(3), col.NumDocuments)
}

func Test_Index(t *testing.T) {
	type Expected struct {
		count   int
		wantErr bool
		Err     error
	}

	type Args struct {
		name     string
		person   Person
		expected Expected
	}

	tests := []Args{
		{
			name: "Successfully index the document",
			person: Person{
				ID:        "uid-5",
				Age:       5,
				Name:      "emmanuella",
				CreatedAt: "2022-09-02T15:04:05+01:00",
				UpdatedAt: "2022-09-02T15:04:05+01:00",
			},
			expected: Expected{
				count:   1,
				wantErr: false,
			},
		},
		{
			name: "Should fail to index the document - missing id field",
			person: Person{
				Age:       5,
				Name:      "emmanuella",
				CreatedAt: "2022-09-02T15:04:05+01:00",
				UpdatedAt: "2022-09-02T15:04:05+01:00",
			},
			expected: Expected{
				count:   1,
				wantErr: true,
				Err:     ErrIDFieldIsRequired,
			},
		},
		{
			name: "Should fail to index the document - missing created_at field",
			person: Person{
				Age:       5,
				ID:        "uid-2",
				Name:      "emmanuella",
				UpdatedAt: "2022-09-02T15:04:05+01:00",
			},
			expected: Expected{
				count:   1,
				wantErr: true,
				Err:     ErrCreatedAtFieldIsRequired,
			},
		},
		{
			name: "Should fail to index the document - missing updated_at field",
			person: Person{
				Age:       5,
				ID:        "uid-2",
				Name:      "emmanuella",
				CreatedAt: "2022-09-02T15:04:05+01:00",
			},
			expected: Expected{
				count:   1,
				wantErr: true,
				Err:     ErrUpdatedAtFieldIsRequired,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := NewTypesenseClient(getTypesenseHost(), getTypesenseAPIKey())
			require.NoError(t, err)
			defer deleteCollection(t, ts, testCollection)

			var doc convoy.GenericMap
			err = tt.person.toGenericMap(&doc)
			require.NoError(t, err)

			err = ts.Index(testCollection, doc)
			if tt.expected.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.expected.Err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_Search(t *testing.T) {
	ts, err := NewTypesenseClient(getTypesenseHost(), getTypesenseAPIKey())
	require.NoError(t, err)
	defer deleteCollection(t, ts, testCollection)

	people := []Person{
		{
			ID:        "uid-1",
			Age:       1,
			Name:      "subomi",
			CreatedAt: "2022-09-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			ID:        "uid-2",
			Age:       2,
			Name:      "raymond",
			CreatedAt: "2022-08-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			ID:        "uid-3",
			Age:       2,
			Name:      "emmanuel",
			CreatedAt: "2022-08-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			ID:        "uid-4",
			Age:       3,
			Name:      "pelumi",
			CreatedAt: "2022-09-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			ID:        "uid-5",
			Age:       5,
			Name:      "emmanuella",
			CreatedAt: "2022-09-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
	}

	// seed the search db
	for _, e := range people {
		var doc convoy.GenericMap
		err = e.toGenericMap(&doc)
		require.NoError(t, err)

		err = ts.Index(testCollection, doc)
		require.NoError(t, err)
	}

	type Expected struct {
		count int
		ids   []string
	}

	type Args struct {
		name     string
		query    string
		expected Expected
	}

	tests := []Args{
		{
			name:  "search for one record by the 'name' field",
			query: "ray",
			expected: Expected{
				count: 1,
				ids:   []string{"uid-2"},
			},
		},
		{
			name:  "search for two records by the 'name' field",
			query: "emma",
			expected: Expected{
				count: 2,
				ids:   []string{"uid-5", "uid-3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterByBuilder := new(strings.Builder)
			filterByBuilder.WriteString("created_at:=[0..10000000000]")

			pp, _, err := ts.Search(testCollection, &datastore.SearchFilter{
				Query:    tt.query,
				FilterBy: filterByBuilder.String(),
				Pageable: datastore.Pageable{Page: 1, PerPage: 10, Sort: 1},
			})
			require.NoError(t, err)

			require.Equal(t, tt.expected.count, len(pp))
			for i, v := range pp {
				require.Equal(t, tt.expected.ids[i], v["id"].(string))
			}
		})
	}
}
