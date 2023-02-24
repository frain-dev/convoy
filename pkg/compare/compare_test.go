package compare

import (
	"testing"

	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/stretchr/testify/require"
)

func TestCompare(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]interface{}
		filter  map[string]interface{}
		want    bool
	}{
		{
			name: "equal",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 5,
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 5,
				},
			},
			want: true,
		},
		{
			name: "equal with operator - number",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 5,
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"age": map[string]interface{}{
						"$eq": 5,
					},
				},
			},
			want: true,
		},
		{
			name: "equal with operator - string",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"name": "tunde",
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"name": map[string]interface{}{
						"$eq": "tunde",
					},
				},
			},
			want: true,
		},
		{
			name: "not equal - false",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 5,
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"age": map[string]interface{}{
						"$neq": 5,
					},
				},
			},
			want: false,
		},
		{
			name: "not equal - true",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 11,
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"age": map[string]interface{}{
						"$neq": 5,
					},
				},
			},
			want: true,
		},
		{
			name: "less than - true",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 11,
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"age": map[string]interface{}{
						"$lt": 15,
					},
				},
			},
			want: true,
		},
		{
			name: "less than - false",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 11,
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"age": map[string]interface{}{
						"$lt": 5,
					},
				},
			},
			want: false,
		},
		{
			name: "greater than - true",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 11,
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"age": map[string]interface{}{
						"$gt": 5,
					},
				},
			},
			want: true,
		},
		{
			name: "greater than - false",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 11,
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"age": map[string]interface{}{
						"$gt": 50,
					},
				},
			},
			want: false,
		},
		{
			name: "in array - false",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"name": "raymond",
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"name": map[string]interface{}{
						"$in": []interface{}{"subomi", "daniel"},
					},
				},
			},
			want: false,
		},
		{
			name: "in array - true",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"name": "subomi",
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"name": map[string]interface{}{
						"$in": []interface{}{"subomi", "daniel"},
					},
				},
			},
			want: true,
		},
		{
			name: "not in array - true",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"name": "raymond",
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"name": map[string]interface{}{
						"$nin": []interface{}{"subomi", "daniel"},
					},
				},
			},
			want: true,
		},
		{
			name: "not in array - false",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"name": "subomi",
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"name": map[string]interface{}{
						"$nin": []interface{}{"subomi", "daniel"},
					},
				},
			},
			want: false,
		},
		{
			name: "query array value - true",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"name": []interface{}{"subomi", "daniel"},
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"name": "subomi",
				},
			},
			want: true,
		},
		{
			name: "query array value - false",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"name": []interface{}{"subomi", "daniel"},
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"name": "raymond",
				},
			},
			want: false,
		},
		{
			name: "$and and $or",
			payload: map[string]interface{}{
				"cities": []interface{}{
					"lagos",
					"ibadan",
					"agodi",
				},
				"type": "weekly",
				"temperatures": []interface{}{
					30,
					12,
					39.9,
					10,
				},
				"person": map[string]interface{}{
					"age": 12,
				},
			},
			filter: map[string]interface{}{
				"$and": []interface{}{
					map[string]interface{}{
						"age": map[string]interface{}{
							"$gte": 10,
						},
					},
					map[string]interface{}{
						"$or": []interface{}{
							map[string]interface{}{
								"type": "weekly",
							},
							map[string]interface{}{
								"cities": "lagos",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "$exist - true",
			payload: map[string]interface{}{
				"cities": []interface{}{
					"lagos",
					"ibadan",
					"agodi",
				},
				"type": "weekly",
				"temperatures": []interface{}{
					30,
					12,
					39.9,
					10,
				},
				"person": map[string]interface{}{
					"age": 12,
				},
			},
			filter: map[string]interface{}{
				"person": map[string]interface{}{
					"age": map[string]interface{}{
						"$exist": true,
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := flatten.Flatten(tt.payload)
			require.NoError(t, err)

			f, err := flatten.Flatten(tt.filter)
			require.NoError(t, err)

			matched := Compare(p, f)
			require.Equal(t, tt.want, matched)
		})
	}
}
