package compare

import (
	"encoding/json"
	"testing"

	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/nsf/jsondiff"
)

func TestCompare(t *testing.T) {
	tests := []struct {
		name    string
		payload interface{}
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
		{
			name: "array operator ($.) -  root level",
			payload: []interface{}{
				map[string]interface{}{
					"event": "meetup",
				},
				map[string]interface{}{
					"venues": []interface{}{
						map[string]interface{}{
							"lagos": []interface{}{
								"ikeja",
								"lekki",
							},
						},
						map[string]interface{}{
							"ibadan": []interface{}{
								"bodija",
								"dugbe",
							},
						},
					},
				},
				map[string]interface{}{
					"speakers": []interface{}{
						map[string]interface{}{
							"name": "raymond",
						},
						map[string]interface{}{
							"name": "subomi",
						},
					},
				},
			},
			filter: map[string]interface{}{
				"$.venues.$.lagos": "ikeja",
			},
			want: true,
		},
		{
			name: "array operator ($.) -  1 level",
			payload: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"event": "meetup",
					},
					map[string]interface{}{
						"venue": "test",
					},
				},
				"speakers": []interface{}{
					"raymond",
					"subomi",
				},
				"swag": "hoodies",
			},
			filter: map[string]interface{}{
				"data.$.event": "meetup",
				"data.$.venue": "test",
			},
			want: true,
		},
		{
			name: "nested array operator ($.) -  2 levels",
			payload: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"event": "meetup",
					},
					map[string]interface{}{
						"venue": "test",
					},
					map[string]interface{}{
						"speakers": []interface{}{
							map[string]interface{}{
								"name": "raymond",
							},
							map[string]interface{}{
								"name": "subomi",
							},
						},
					},
				},
				"swag": "hoodies",
			},
			filter: map[string]interface{}{
				"data.$.speakers.$.name": "raymond",
				"swag":                   "hoodies",
			},
			want: true,
		},
		{
			name: "nested array operator ($.) - 3 levels",
			payload: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"event": "meetup",
					},
					map[string]interface{}{
						"venue": "test",
					},
					map[string]interface{}{
						"speakers": []interface{}{
							map[string]interface{}{
								"name": "raymond",
							},
							map[string]interface{}{
								"name": "subomi",
							},
						},
					},
				},
				"swag": "hoodies",
			},

			filter: map[string]interface{}{
				"data.$.speakers.$.name": "raymond",
				"swag":                   "hoodies",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := flatten.Flatten(tt.payload)
			if err != nil {
				t.Errorf("failed to flatten JSON: %v", err)
			}

			f, err := flatten.Flatten(tt.filter)
			if err != nil {
				t.Errorf("failed to flatten JSON: %v", err)
			}

			matched := Compare(p, f)

			if !jsonEqual(matched, tt.want) {
				t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", matched, tt.want)
			}
		})
	}
}

func jsonEqual(got, want interface{}) bool {
	var a, b []byte
	a, _ = json.Marshal(got)
	b, _ = json.Marshal(want)

	diff, _ := jsondiff.Compare(a, b, &jsondiff.Options{})
	return diff == jsondiff.FullMatch
}
