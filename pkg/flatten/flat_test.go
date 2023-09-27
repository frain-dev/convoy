package flatten

import (
	_ "embed"
	"encoding/json"
	"errors"
	"testing"

	"github.com/nsf/jsondiff"
)

//go:embed gh_event.json
var ghEvent []byte

//go:embed gh_event_flat.json
var ghEventFlat []byte

func TestFlattenMap(t *testing.T) {
	tests := []struct {
		name  string
		given interface{}
		want  map[string]interface{}
		err   error
	}{
		/////////////////// string
		{
			name:  "string value",
			given: map[string]interface{}{"name": "ukpe"},
			want:  map[string]interface{}{"name": "ukpe"},
		},
		{
			name:  "nested string value",
			given: map[string]interface{}{"$.venues.$.lagos": "lekki"},
			want:  map[string]interface{}{"$.venues.$.lagos": "lekki"},
		},
		{
			name:  "invalid operator",
			given: map[string]interface{}{"$venues": "bariga"},
			err:   errors.New("$venues starts with a $ and is not a valid operator"),
		},
		{
			name:  "weird case",
			given: map[string]interface{}{"$$$$$": "lmao"},
			err:   errors.New("$$$$$ starts with a $ and is not a valid operator"),
		},
		{
			name:  "string value with trailing $",
			given: map[string]interface{}{"lagos$": "lekki"},
			want:  map[string]interface{}{"lagos$": "lekki"},
		},
		{
			name:  "nested string value - trailing .$",
			given: map[string]interface{}{"$.venues.$.lagos.$": "lekki"},
			want:  map[string]interface{}{"$.venues.$.lagos.$": "lekki"},
		},
		{
			name:  "nested string value - trailing .$ with inner operator",
			given: map[string]interface{}{"$.venues.$lagos.$": "lekki"},
			want:  map[string]interface{}{"$.venues.$lagos.$": "lekki"},
		},
		{
			name:  "nested string value - trailing .$ with invalid operator prefix",
			given: map[string]interface{}{"$venues.$.lagos.$": "bariga"},
			err:   errors.New("$venues.$.lagos.$ starts with a $ and is not a valid operator"),
		},
		{
			name:  "empty map",
			given: map[string]interface{}{},
			want:  map[string]interface{}{},
		},
		{
			name:  "empty array",
			given: []interface{}{},
			want:  map[string]interface{}{},
		},
		{
			name:  "nothing",
			given: nil,
			want:  map[string]interface{}{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Flatten(tt.given)
			if tt.err != nil {
				if tt.err.Error() != err.Error() {
					t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", err.Error(), tt.err.Error())
				}
				return
			}

			if !jsonEqual(got, tt.want) {
				t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

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
			name:  "string value",
			given: `{"$.name": "ukpe"}`,
			want:  map[string]interface{}{"$.name": "ukpe"},
		},
		{
			name:  "string value",
			given: `{"$.venues.$.lagos": "lekki"}`,
			want:  map[string]interface{}{"$.venues.$.lagos": "lekki"},
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
			var given interface{}
			err := json.Unmarshal([]byte(test.given), &given)
			if err != nil {
				t.Errorf("failed to unmarshal JSON: %v", err)
			}

			got, err := Flatten(given)
			if err != nil {
				t.Errorf("failed to flatten: %+v", err)
			}

			if !jsonEqual(got, test.want) {
				t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", got, test.want)
			}
		})
	}
}

func TestFlattenWithPrefix(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  map[string]interface{}
	}{
		/////////////////// string
		{
			name:  "string value",
			given: `{"hello": "world"}`,
			want:  map[string]interface{}{"data.hello": "world"},
		},
		{
			name:  "nested string value",
			given: `{"hello":{"world":"good morning"}}`,
			want:  map[string]interface{}{"data.hello.world": "good morning"},
		},
		{
			name:  "double nested string value",
			given: `{"hello":{"world":{"again":"good morning"}}}`,
			want:  map[string]interface{}{"data.hello.world.again": "good morning"},
		},

		/////////////////// float
		{
			name:  "float",
			given: `{"hello": 1234.99}`,
			want:  map[string]interface{}{"data.hello": 1234.99},
		},
		{
			name:  "nested float value",
			given: `{"hello":{"world":1234.99}}`,
			want:  map[string]interface{}{"data.hello.world": 1234.99},
		},

		/////////////////// boolean
		{
			name:  "boolean value",
			given: `{"hello": true}`,
			want:  map[string]interface{}{"data.hello": true},
		},
		{
			name:  "nested boolean",
			given: `{"hello":{"world":true}}`,
			want:  map[string]interface{}{"data.hello.world": true},
		},

		/////////////////// nil
		{
			name:  "nil value",
			given: `{"hello": null}`,
			want:  map[string]interface{}{"data.hello": nil},
		},
		{
			name:  "nested nil value",
			given: `{"hello":{"world":null}}`,
			want:  map[string]interface{}{"data.hello.world": nil},
		},

		/////////////////// map
		{
			name:  "empty value",
			given: `{"hello":{}}`,
			want:  map[string]interface{}{"data.hello": map[string]interface{}{}},
		},
		{
			name:  "empty object",
			given: `{"hello":{"empty":{"nested":{}}}}`,
			want:  map[string]interface{}{"data.hello.empty.nested": map[string]interface{}{}},
		},

		/////////////////// slice
		{
			name:  "empty slice",
			given: `{"hello":[]}`,
			want:  map[string]interface{}{"data.hello": []interface{}{}},
		},
		{
			name:  "nested empty slice",
			given: `{"hello":{"world":[]}}`,
			want:  map[string]interface{}{"data.hello.world": []interface{}{}},
		},
		{
			name:  "nested slice",
			given: `{"hello":{"world":["one","two"]}}`,
			want: map[string]interface{}{
				"data.hello.world": []interface{}{"one", "two"},
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
				"data.hello.lorem.ipsum": "again",
				"data.hello.lorem.dolor": "sit",
				"data.world.lorem.ipsum": "again",
				"data.world.lorem.dolor": "sit",
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
				"data.hallo.lorem":       []interface{}{"10", "1"},
				"data.hallo.ipsum.dolor": []interface{}{"1", "10"},
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
				"data.hallo.lorem":       []interface{}{float64(10), float64(1)},
				"data.hallo.ipsum.dolor": []interface{}{float64(1), float64(10)},
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
				"data.hallo.lorem":       []interface{}{float64(10), float64(1)},
				"data.hallo.ipsum.dolor": []interface{}{"1", "10"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var given interface{}
			err := json.Unmarshal([]byte(test.given), &given)
			if err != nil {
				t.Errorf("failed to unmarshal JSON: %v", err)
			}

			got, err := FlattenWithPrefix("data", given)
			if err != nil {
				t.Errorf("failed to flatten: %+v", err)
			}

			if !jsonEqual(got, test.want) {
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

			if !jsonEqual(got, test.want) {
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

			if !jsonEqual(got, test.want) {
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

			if !jsonEqual(got, test.want) {
				t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", got, test.want)
			}
		})
	}
}

func TestFlattenArray(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  interface{}
	}{

		/////////////////// arrays
		{
			name: "array of numbers and strings",
			given: `[
				{
					  "hallo": {
						  "lorem": [20, 2],
						  "ipsum": {
							  "dolor": ["1", "10"]
						  }
					  }
				  },
				  {
					  "game": {
						  "name": "Elden Ring",
						  "authors": {
							  "first_name": "George",
							  "last_name": "Martin"
						  }
					  }
				  },
				  {
					  "person": {
						  "ages": [100, -1],
						  "names": {
							  "parts": ["ray", "mond"]
						  }
					  }
				  }
			  ]`,
			want: map[string]interface{}{
				"0.hallo.lorem":             []interface{}{float64(20), float64(2)},
				"0.hallo.ipsum.dolor":       []interface{}{"1", "10"},
				"1.game.authors.first_name": "George",
				"1.game.authors.last_name":  "Martin",
				"1.game.name":               "Elden Ring",
				"2.person.ages":             []interface{}{float64(100), float64(-1)},
				"2.person.names.parts":      []interface{}{"ray", "mond"},
			},
		},
		{
			name: "flatten nested array",
			given: `{
				"data": [
				  {
					  "event" : "meetup"
				  },
						  {
					  "venue" : "test"
				  }
				],
				"speakers": ["raymond", "subomi"],
				"swag": "hoodies"
			}`,
			want: map[string]interface{}{
				"data.0.event": "meetup",
				"data.1.venue": "test",
				"speakers": []interface{}{
					"raymond",
					"subomi",
				},
				"swag": "hoodies",
			},
		},
		{
			name: "should not affect $and and $or",
			given: `{
				"$and": [
				  {
					"age": {
					  "$gte": 10
					}
				  },
				  {
					"$or": [
					  {
						"type": "weekly"
					  },
					  {
						"cities": "lagos"
					  }
					]
				  }
				]
			  }`,
			want: map[string]interface{}{
				"$and": []interface{}{
					map[string]interface{}{
						"age": map[string]interface{}{
							"$gte": float64(10),
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
		},
		{
			name: "flatten nested arrays",
			given: `{
				"data": [
				  {
					"event": "meetup"
				  },
				  {
					"venue": "test"
				  },
				  {
					"speakers": [
					  {
						"name": "raymond"
					  },
					  {
						"name": "subomi"
					  }
					]
				  }
				],
				"swag": "hoodies"
			  }`,
			want: map[string]interface{}{
				"data.0.event":           "meetup",
				"data.1.venue":           "test",
				"data.2.speakers.0.name": "raymond",
				"data.2.speakers.1.name": "subomi",
				"swag":                   "hoodies",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var given interface{}
			err := json.Unmarshal([]byte(test.given), &given)
			if err != nil {
				t.Errorf("failed to unmarshal JSON: %v", err)
			}

			got, err := Flatten(given)
			if err != nil {
				t.Errorf("failed to flatten: %+v", err)
			}

			if !jsonEqual(got, test.want) {
				t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", got, test.want)
			}
		})
	}
}

func TestFlattenLargeJSON(t *testing.T) {
	var given, want interface{}
	err := json.Unmarshal([]byte(ghEvent), &given)
	if err != nil {
		t.Errorf("failed to unmarshal JSON: %v", err)
	}

	err = json.Unmarshal([]byte(ghEventFlat), &want)
	if err != nil {
		t.Errorf("failed to unmarshal JSON: %v", err)
	}

	got, err := Flatten(given)
	if err != nil {
		t.Errorf("failed to flatten: %+v", err)
	}

	if !jsonEqual(got, want) {
		expectedJson, _ := json.MarshalIndent(got, "", " ")
		t.Errorf("%v\n", string(expectedJson))
	}
}

func jsonEqual(got, want interface{}) bool {
	var a, b []byte
	a, _ = json.Marshal(got)
	b, _ = json.Marshal(want)

	diff, _ := jsondiff.Compare(a, b, &jsondiff.Options{})
	return diff == jsondiff.FullMatch
}
