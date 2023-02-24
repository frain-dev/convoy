package flatten

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFlatten(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  map[string]interface{}
	}{
		/////////////////// string
		{
			name:  "string value",
			given: `{"hello": "world"}`,
			want:  map[string]interface{}{"hello": "world"},
		},
		{
			name:  "nested string value",
			given: `{"hello":{"world":"good morning"}}`,
			want:  map[string]interface{}{"hello.world": "good morning"},
		},
		{
			name:  "double nested string value",
			given: `{"hello":{"world":{"again":"good morning"}}}`,
			want:  map[string]interface{}{"hello.world.again": "good morning"},
		},

		/////////////////// float
		{
			name:  "float",
			given: `{"hello": 1234.99}`,
			want:  map[string]interface{}{"hello": 1234.99},
		},
		{
			name:  "nested float value",
			given: `{"hello":{"world":1234.99}}`,
			want:  map[string]interface{}{"hello.world": 1234.99},
		},

		/////////////////// boolean
		{
			name:  "boolean value",
			given: `{"hello": true}`,
			want:  map[string]interface{}{"hello": true},
		},
		{
			name:  "nested boolean",
			given: `{"hello":{"world":true}}`,
			want:  map[string]interface{}{"hello.world": true},
		},

		/////////////////// nil
		{
			name:  "nil value",
			given: `{"hello": null}`,
			want:  map[string]interface{}{"hello": nil},
		},
		{
			name:  "nested nil value",
			given: `{"hello":{"world":null}}`,
			want:  map[string]interface{}{"hello.world": nil},
		},

		/////////////////// map
		{
			name:  "empty value",
			given: `{"hello":{}}`,
			want:  map[string]interface{}{"hello": map[string]interface{}{}},
		},
		{
			name:  "empty object",
			given: `{"hello":{"empty":{"nested":{}}}}`,
			want:  map[string]interface{}{"hello.empty.nested": map[string]interface{}{}},
		},

		/////////////////// slice
		{
			name:  "empty slice",
			given: `{"hello":[]}`,
			want:  map[string]interface{}{"hello": []interface{}{}},
		},
		{
			name:  "nested empty slice",
			given: `{"hello":{"world":[]}}`,
			want:  map[string]interface{}{"hello.world": []interface{}{}},
		},
		{
			name:  "nested slice",
			given: `{"hello":{"world":["one","two"]}}`,
			want: map[string]interface{}{
				"hello.world": []interface{}{"one", "two"},
			},
		},

		/////////////////// combos
		{
			name: "multiple keys",
			given: `{
				"hello": {
					"lorem": {
						"ipsum": "again",
						"dolor": "sit"
					}
				},
				"world": {
					"lorem": {
						"ipsum": "again",
						"dolor": "sit"
					}
				}
			}`,
			want: map[string]interface{}{
				"hello.lorem.ipsum": "again",
				"hello.lorem.dolor": "sit",
				"world.lorem.ipsum": "again",
				"world.lorem.dolor": "sit",
			},
		},

		/////////////////// nested slices
		{
			name: "array of strings",
			given: `{
				"hallo": {
					"lorem": ["10", "1"],
					"ipsum": {
						"dolor": ["1", "10"]
					}
				}
			}`,
			want: map[string]interface{}{
				"hallo.lorem":       []interface{}{"10", "1"},
				"hallo.ipsum.dolor": []interface{}{"1", "10"},
			},
		},
		{
			name: "array of integers",
			given: `{
				"hallo": {
					"lorem": [10, 1],
					"ipsum": {
						"dolor": [1, 10]
					}
				}
			}`,
			want: map[string]interface{}{
				"hallo.lorem":       []interface{}{float64(10), float64(1)},
				"hallo.ipsum.dolor": []interface{}{float64(1), float64(10)},
			},
		},

		/////////////////// slice combos
		{
			name: "array of numbers and strings",
			given: `{
				"hallo": {
					"lorem": [10, 1],
					"ipsum": {
						"dolor": ["1", "10"]
					}
				}
			}`,
			want: map[string]interface{}{
				"hallo.lorem":       []interface{}{float64(10), float64(1)},
				"hallo.ipsum.dolor": []interface{}{"1", "10"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var given map[string]interface{}
			err := json.Unmarshal([]byte(test.given), &given)
			if err != nil {
				t.Errorf("failed to unmarshal JSON: %v", err)
			}

			got, err := Flatten(given)
			if err != nil {
				t.Errorf("failed to flatten: %+v", err)
			}

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", got, test.want)
			}
		})
	}
}

func TestFlattenWithOperator(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  map[string]interface{}
	}{
		/////////////////// string operator
		{
			name:  "nested string value",
			given: `{"name":{"$eq":"raymond"}}`,
			want: map[string]interface{}{
				"name": map[string]interface{}{
					"$eq": "raymond",
				},
			},
		},
		{
			name: "nested string value",
			given: `{
				"filter": {
					"person": {
						"age": {
							"$eq": 5
						}
					}
				}
			}`,
			want: map[string]interface{}{
				"filter.person.age": map[string]interface{}{
					"$eq": float64(5),
				},
			},
		},

		/////////////////// number operator
		{
			name:  "double nested string value",
			given: `{"person":{"age":{"$gte":10}}}`,
			want: map[string]interface{}{
				"person.age": map[string]interface{}{
					"$gte": float64(10),
				},
			},
		},

		/////////////////// array operator
		{
			name:  "double nested string value",
			given: `{"person":{"age":{"$in":[10, 20]}}}`,
			want: map[string]interface{}{
				"person.age": map[string]interface{}{
					"$in": []interface{}{float64(10), float64(20)},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var given map[string]interface{}
			err := json.Unmarshal([]byte(test.given), &given)
			if err != nil {
				t.Errorf("failed to unmarshal JSON: %v", err)
			}

			got, err := Flatten(given)
			if err != nil {
				t.Errorf("failed to flatten: %+v", err)
			}

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", got, test.want)
			}
		})
	}
}

func TestFlattenWithOperatorAndMaps(t *testing.T) {
	tests := []struct {
		name  string
		given map[string]interface{}
		want  map[string]interface{}
	}{
		/////////////////// string operator
		{
			name: "nested string value",
			given: map[string]interface{}{
				"name": map[string]interface{}{
					"$eq": "raymond",
				},
			},
			want: map[string]interface{}{
				"name": map[string]interface{}{
					"$eq": "raymond",
				},
			},
		},
		{
			name: "nested string value",
			given: map[string]interface{}{
				"filter": map[string]interface{}{
					"person": map[string]interface{}{
						"age": map[string]interface{}{
							"$eq": float64(5),
						},
					},
				},
			},
			want: map[string]interface{}{
				"filter.person.age": map[string]interface{}{
					"$eq": float64(5),
				},
			},
		},

		/////////////////// number operator
		{
			name: "double nested string value",
			given: map[string]interface{}{
				"person": map[string]interface{}{
					"age": map[string]interface{}{
						"$gte": float64(10),
					},
				},
			},
			want: map[string]interface{}{
				"person.age": map[string]interface{}{
					"$gte": float64(10),
				},
			},
		},

		/////////////////// array operator
		{
			name: "double nested string value",
			given: map[string]interface{}{
				"person": map[string]interface{}{
					"age": map[string]interface{}{
						"$in": []interface{}{float64(10), float64(20)},
					},
				},
			},
			want: map[string]interface{}{
				"person.age": map[string]interface{}{
					"$in": []interface{}{float64(10), float64(20)},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Flatten(test.given)
			if err != nil {
				t.Errorf("failed to flatten: %+v", err)
			}

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", got, test.want)
			}
		})
	}
}

func TestFlattenWithOrAndOperator(t *testing.T) {
	tests := []struct {
		name    string
		given   map[string]interface{}
		want    map[string]interface{}
		wantErr bool
		err     error
	}{
		/////////////////// $or operator
		{
			name: "top level $or operator - nothing to flatten",
			given: map[string]interface{}{
				"$or": []map[string]interface{}{
					{
						"age": map[string]interface{}{
							"$in": []int{10, 11, 12},
						},
					},
					{
						"temperatures": 39.9,
					},
				},
			},
			want: map[string]interface{}{
				"$or": []map[string]interface{}{
					{
						"age": map[string]interface{}{
							"$in": []int{10, 11, 12},
						},
					},
					{
						"temperatures": 39.9,
					},
				},
			},
		},

		{
			name: "top level $or operator - flatten children",
			given: map[string]interface{}{
				"$or": []map[string]interface{}{
					{
						"person": map[string]interface{}{
							"age": map[string]interface{}{
								"$in": []int{10, 11, 12},
							},
						},
					},
					{
						"places": map[string]interface{}{
							"temperatures": 39.9,
						},
					},
				},
			},
			want: map[string]interface{}{
				"$or": []map[string]interface{}{
					{
						"person.age": map[string]interface{}{
							"$in": []int{10, 11, 12},
						},
					},
					{
						"places.temperatures": 39.9,
					},
				},
			},
		},

		{
			name: "top level $or operator - flatten children",
			given: map[string]interface{}{
				"$or": []map[string]interface{}{
					{
						"person": map[string]interface{}{
							"age": map[string]interface{}{
								"$in": []int{10, 11, 12},
							},
						},
					},
					{
						"places": map[string]interface{}{
							"temperatures": 39.9,
						},
					},
				},
			},
			want: map[string]interface{}{
				"$or": []map[string]interface{}{
					{
						"person.age": map[string]interface{}{
							"$in": []int{10, 11, 12},
						},
					},
					{
						"places.temperatures": 39.9,
					},
				},
			},
		},

		{
			name: "top level $or operator - or is not an array",
			given: map[string]interface{}{
				"$or": map[string]interface{}{
					"person": map[string]interface{}{
						"age": map[string]interface{}{
							"$in": []int{10, 11, 12},
						},
					},
					"places": map[string]interface{}{
						"temperatures": 39.9,
					},
				},
			},
			wantErr: true,
			err:     ErrOrAndMustBeArray,
		},

		/////////////////// $and operator
		{
			name: "top level $and operator - nothing to flatten",
			given: map[string]interface{}{
				"$and": []map[string]interface{}{
					{
						"age": map[string]interface{}{
							"$in": []int{10, 11, 12},
						},
					},
					{
						"temperatures": 39.9,
					},
				},
			},
			want: map[string]interface{}{
				"$and": []map[string]interface{}{
					{
						"age": map[string]interface{}{
							"$in": []int{10, 11, 12},
						},
					},
					{
						"temperatures": 39.9,
					},
				},
			},
		},

		{
			name: "top level $and operator - flatten children",
			given: map[string]interface{}{
				"$and": []map[string]interface{}{
					{
						"person": map[string]interface{}{
							"age": map[string]interface{}{
								"$in": []int{10, 11, 12},
							},
						},
					},
					{
						"places": map[string]interface{}{
							"temperatures": 39.9,
						},
					},
				},
			},
			want: map[string]interface{}{
				"$and": []map[string]interface{}{
					{
						"person.age": map[string]interface{}{
							"$in": []int{10, 11, 12},
						},
					},
					{
						"places.temperatures": 39.9,
					},
				},
			},
		},

		{
			name: "top level $and operator - flatten children",
			given: map[string]interface{}{
				"$and": []map[string]interface{}{
					{
						"person": map[string]interface{}{
							"age": map[string]interface{}{
								"$in": []int{10, 11, 12},
							},
						},
					},
					{
						"places": map[string]interface{}{
							"temperatures": 39.9,
						},
					},
				},
			},
			want: map[string]interface{}{
				"$and": []map[string]interface{}{
					{
						"person.age": map[string]interface{}{
							"$in": []int{10, 11, 12},
						},
					},
					{
						"places.temperatures": 39.9,
					},
				},
			},
		},

		{
			name: "top level $and operator - or is not an array",
			given: map[string]interface{}{
				"$and": map[string]interface{}{
					"person": map[string]interface{}{
						"age": map[string]interface{}{
							"$in": []int{10, 11, 12},
						},
					},
					"places": map[string]interface{}{
						"temperatures": 39.9,
					},
				},
			},
			wantErr: true,
			err:     ErrOrAndMustBeArray,
		},

		/////////////////// combine $and and $or operators
		{
			name: "combine $and and $or operators - nothing to flatten",
			given: map[string]interface{}{
				"$and": []map[string]interface{}{
					{
						"$or": []map[string]interface{}{
							{
								"age": map[string]interface{}{
									"$in": []int{10, 11, 12},
								},
							},
							{
								"temperatures": 39.9,
							},
						},
					},
					{
						"temperatures": 39.9,
					},
				},
			},
			want: map[string]interface{}{
				"$and": []map[string]interface{}{
					{
						"$or": []map[string]interface{}{
							{
								"age": map[string]interface{}{
									"$in": []int{10, 11, 12},
								},
							},
							{
								"temperatures": 39.9,
							},
						},
					},
					{
						"temperatures": 39.9,
					},
				},
			},
		},

		{
			name: "combine $and and $or operators - nothing to flatten",
			given: map[string]interface{}{
				"$and": []map[string]interface{}{
					{
						"$or": []map[string]interface{}{
							{
								"person": map[string]interface{}{
									"age": map[string]interface{}{
										"$in": []int{10, 11, 12},
									},
								},
							},
							{
								"places": map[string]interface{}{
									"temperatures": 39.9,
								},
							},
						},
					},
					{
						"city": "lagos",
					},
				},
			},
			want: map[string]interface{}{
				"$and": []map[string]interface{}{
					{
						"$or": []map[string]interface{}{
							{
								"person.age": map[string]interface{}{
									"$in": []int{10, 11, 12},
								},
							},
							{
								"places.temperatures": 39.9,
							},
						},
					},
					{
						"city": "lagos",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Flatten(test.given)
			if test.wantErr {
				if test.err != err {
					t.Errorf("failed to flatten: %+v", err)
				}
			}

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", got, test.want)
			}
		})
	}
}
