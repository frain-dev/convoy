//go:build integration
// +build integration

package typesense

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

const testCollection = "test"

type Person struct {
	ID        string `json:"id,omitempty"`
	UID       string `json:"uid,omitempty"`
	Name      string `json:"name,omitempty"`
	Age       int    `json:"age,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func toGenericMap(p Person, document *map[string]interface{}) error {
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
	return os.Getenv("TEST_TYPESENSE_HOST")
}

func getTypesenseAPIKey() string {
	return os.Getenv("TEST_TYPESENSE_API_KEY")
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
		ID:        ulid.Make().String(),
		UID:       "uid-1",
		CreatedAt: "2022-08-02T15:04:05+01:00",
		UpdatedAt: "2022-09-02T15:04:05+01:00",
	}

	var doc map[string]interface{}
	err = toGenericMap(p, &doc)
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
			UID:       "uid-1",
			ProjectID: "project-1",
			ID:        ulid.Make().String(),
			CreatedAt: "2022-09-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			Age:       2,
			ProjectID: "project-1",
			Name:      "raymond",
			ID:        ulid.Make().String(),
			UID:       "uid-2",
			CreatedAt: "2022-08-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			Age:       2,
			ID:        ulid.Make().String(),
			Name:      "emmanuel",
			ProjectID: "project-1",
			UID:       "uid-3",
			CreatedAt: "2022-08-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
	}

	for _, p := range people {
		var doc map[string]interface{}
		err = toGenericMap(p, &doc)
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
				ID:        ulid.Make().String(),
				UID:       "uid-5",
				Age:       5,
				ProjectID: "project-1",
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
				ProjectID: "project-1",
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
				ID:        ulid.Make().String(),
				Age:       5,
				ProjectID: "project-1",
				UID:       "uid-2",
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
				ID:        ulid.Make().String(),
				ProjectID: "project-1",
				UID:       "uid-2",
				Name:      "emmanuella",
				CreatedAt: "2022-09-02T15:04:05+01:00",
			},
			expected: Expected{
				count:   1,
				wantErr: true,
				Err:     ErrUpdatedAtFieldIsRequired,
			},
		},
		{
			name: "Should fail to index the document - missing uid field",
			person: Person{
				Age:       5,
				ID:        ulid.Make().String(),
				ProjectID: "project-1",
				Name:      "emmanuella",
				CreatedAt: "2022-09-02T15:04:05+01:00",
			},
			expected: Expected{
				count:   1,
				wantErr: true,
				Err:     ErrUidFieldIsRequired,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := NewTypesenseClient(getTypesenseHost(), getTypesenseAPIKey())
			require.NoError(t, err)
			defer deleteCollection(t, ts, testCollection)

			var doc map[string]interface{}
			err = toGenericMap(tt.person, &doc)
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
			UID:       "uid-1",
			ID:        ulid.Make().String(),
			Age:       1,
			ProjectID: "project-1",
			Name:      "subomi",
			CreatedAt: "2022-09-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			ID:        ulid.Make().String(),
			UID:       "uid-2",
			Age:       2,
			ProjectID: "project-1",
			Name:      "raymond",
			CreatedAt: "2022-08-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			ID:        ulid.Make().String(),
			UID:       "uid-3",
			Age:       2,
			ProjectID: "project-1",
			Name:      "emmanuel",
			CreatedAt: "2022-08-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			ID:        ulid.Make().String(),
			UID:       "uid-4",
			ProjectID: "project-1",
			Age:       3,
			Name:      "pelumi",
			CreatedAt: "2022-09-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
		{
			ID:        ulid.Make().String(),
			UID:       "uid-5",
			ProjectID: "project-1",
			Age:       5,
			Name:      "emmanuella",
			CreatedAt: "2022-09-02T15:04:05+01:00",
			UpdatedAt: "2022-09-02T15:04:05+01:00",
		},
	}

	// seed the search db
	for _, e := range people {
		var doc map[string]interface{}
		err = toGenericMap(e, &doc)
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
			pp, _, err := ts.Search(testCollection, &datastore.SearchFilter{
				Query: tt.query,
				FilterBy: datastore.FilterBy{
					ProjectID: "project-1",
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 0,
						CreatedAtEnd:   10000000000,
					},
				},
				Pageable: datastore.Pageable{PerPage: 10},
			})
			require.NoError(t, err)

			require.Equal(t, tt.expected.count, len(pp))
			for i, v := range pp {
				require.Equal(t, tt.expected.ids[i], v)
			}
		})
	}
}
