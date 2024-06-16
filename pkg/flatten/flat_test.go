package flatten

import (
	_ "embed"
	"encoding/json"
	"errors"
	"testing"

	"github.com/nsf/jsondiff"
	"github.com/stretchr/testify/require"
)

//go:embed gh_event.json
var ghEvent []byte

//go:embed gh_event_flat.json
var ghEventFlat []byte

func TestFlattenMap(t *testing.T) {
	tests := []struct {
		name  string
		given interface{}
		want  M
		err   error
	}{
		/////////////////// string
		{
			name:  "string value",
			given: M{"name": "ukpe"},
			want:  M{"name": "ukpe"},
		},
		{
			name:  "nested string value",
			given: M{"$.venues.$.lagos": "lekki"},
			want:  M{"$.venues.$.lagos": "lekki"},
		},
		{
			name:  "invalid operator",
			given: M{"$venues": "bariga"},
			err:   errors.New("$venues starts with a $ and is not a valid operator"),
		},
		{
			name:  "weird case",
			given: M{"$$$$$": "lmao"},
			err:   errors.New("$$$$$ starts with a $ and is not a valid operator"),
		},
		{
			name:  "string value with trailing $",
			given: M{"lagos$": "lekki"},
			want:  M{"lagos$": "lekki"},
		},
		{
			name:  "nested string value - trailing .$",
			given: M{"$.venues.$.lagos.$": "lekki"},
			want:  M{"$.venues.$.lagos.$": "lekki"},
		},
		{
			name:  "nested string value - trailing .$ with inner operator",
			given: M{"$.venues.$lagos.$": "lekki"},
			want:  M{"$.venues.$lagos.$": "lekki"},
		},
		{
			name:  "nested string value - trailing .$ with invalid operator prefix",
			given: M{"$venues.$.lagos.$": "bariga"},
			err:   errors.New("$venues.$.lagos.$ starts with a $ and is not a valid operator"),
		},
		{
			name:  "empty map",
			given: M{},
			want:  M{},
		},
		{
			name:  "empty array",
			given: []interface{}{},
			want:  M{},
		},
		{
			name:  "nothing",
			given: nil,
			want:  M{},
		},
		{
			name:  "string",
			given: "random_string",
			want:  M{},
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
		want  M
	}{
		/////////////////// string
		{
			name:  "string value",
			given: `{"hello": "world"}`,
			want:  M{"hello": "world"},
		},
		{
			name:  "string value",
			given: `{"$.name": "ukpe"}`,
			want:  M{"$.name": "ukpe"},
		},
		{
			name:  "string value",
			given: `{"$.venues.$.lagos": "lekki"}`,
			want:  M{"$.venues.$.lagos": "lekki"},
		},
		{
			name:  "nested string value",
			given: `{"hello":{"world":"good morning"}}`,
			want:  M{"hello.world": "good morning"},
		},
		{
			name:  "double nested string value",
			given: `{"hello":{"world":{"again":"good morning"}}}`,
			want:  M{"hello.world.again": "good morning"},
		},

		/////////////////// float
		{
			name:  "float",
			given: `{"hello": 1234.99}`,
			want:  M{"hello": 1234.99},
		},
		{
			name:  "nested float value",
			given: `{"hello":{"world":1234.99}}`,
			want:  M{"hello.world": 1234.99},
		},

		/////////////////// boolean
		{
			name:  "boolean value",
			given: `{"hello": true}`,
			want:  M{"hello": true},
		},
		{
			name:  "nested boolean",
			given: `{"hello":{"world":true}}`,
			want:  M{"hello.world": true},
		},

		/////////////////// nil
		{
			name:  "nil value",
			given: `{"hello": null}`,
			want:  M{"hello": nil},
		},
		{
			name:  "nested nil value",
			given: `{"hello":{"world":null}}`,
			want:  M{"hello.world": nil},
		},

		/////////////////// map
		{
			name:  "empty value",
			given: `{"hello":{}}`,
			want:  M{"hello": M{}},
		},
		{
			name:  "empty object",
			given: `{"hello":{"empty":{"nested":{}}}}`,
			want:  M{"hello.empty.nested": M{}},
		},

		/////////////////// slice
		{
			name:  "empty slice",
			given: `{"hello":[]}`,
			want:  M{"hello": []interface{}{}},
		},
		{
			name:  "nested empty slice",
			given: `{"hello":{"world":[]}}`,
			want:  M{"hello.world": []interface{}{}},
		},
		{
			name:  "nested slice",
			given: `{"hello":{"world":["one","two"]}}`,
			want: M{
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
			want: M{
				"hello.lorem.ipsum": "again",
				"hello.lorem.dolor": "sit",
				"world.lorem.ipsum": "again",
				"world.lorem.dolor": "sit",
			},
		},

		/////////////////// nested slices
		{
			name: "daniel case of strings",
			given: `{
				"goodbye": "thanks",
				"empty": {},
				"hallo": {
					"lorem": ["10", "1"],
					"floats": [10.44, 1.999],
					"nums": [10, 13],
					"ipsum": {
						"dolor": ["1", "10"],
						"lola": [],
                        "name": "daniel",
                        "age": 14
					}
				}
			}`,
			want: M{
				"goodbye":           "thanks",
				"empty":             M{},
				"hallo.lorem":       []interface{}{"10", "1"},
				"hallo.ipsum.dolor": []interface{}{"1", "10"},
				"hallo.ipsum.lola":  []interface{}{},
				"hallo.floats":      []interface{}{10.44, 1.999},
				"hallo.nums":        []interface{}{10, 13},
				"hallo.ipsum.name":  "daniel",
				"hallo.ipsum.age":   14,
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
			want: M{
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
			want: M{
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
		want  M
	}{
		/////////////////// string
		{
			name:  "string value",
			given: `{"hello": "world"}`,
			want:  M{"data.hello": "world"},
		},
		{
			name:  "nested string value",
			given: `{"hello":{"world":"good morning"}}`,
			want:  M{"data.hello.world": "good morning"},
		},
		{
			name:  "double nested string value",
			given: `{"hello":{"world":{"again":"good morning"}}}`,
			want:  M{"data.hello.world.again": "good morning"},
		},

		/////////////////// float
		{
			name:  "float",
			given: `{"hello": 1234.99}`,
			want:  M{"data.hello": 1234.99},
		},
		{
			name:  "nested float value",
			given: `{"hello":{"world":1234.99}}`,
			want:  M{"data.hello.world": 1234.99},
		},

		/////////////////// boolean
		{
			name:  "boolean value",
			given: `{"hello": true}`,
			want:  M{"data.hello": true},
		},
		{
			name:  "nested boolean",
			given: `{"hello":{"world":true}}`,
			want:  M{"data.hello.world": true},
		},

		/////////////////// nil
		{
			name:  "nil value",
			given: `{"hello": null}`,
			want:  M{"data.hello": nil},
		},
		{
			name:  "nested nil value",
			given: `{"hello":{"world":null}}`,
			want:  M{"data.hello.world": nil},
		},

		/////////////////// map
		{
			name:  "empty value",
			given: `{"hello":{}}`,
			want:  M{"data.hello": M{}},
		},
		{
			name:  "empty object",
			given: `{"hello":{"empty":{"nested":{}}}}`,
			want:  M{"data.hello.empty.nested": M{}},
		},

		/////////////////// slice
		{
			name:  "empty slice",
			given: `{"hello":[]}`,
			want:  M{"data.hello": []interface{}{}},
		},
		{
			name:  "nested empty slice",
			given: `{"hello":{"world":[]}}`,
			want:  M{"data.hello.world": []interface{}{}},
		},
		{
			name:  "nested slice",
			given: `{"hello":{"world":["one","two"]}}`,
			want: M{
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
			want: M{
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
			want: M{
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
			want: M{
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
			want: M{
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
		want  M
	}{
		/////////////////// string operator
		{
			name:  "nested string value",
			given: `{"name":{"$eq":"raymond"}}`,
			want: M{
				"name": M{
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
			want: M{
				"filter.person.age": M{
					"$eq": float64(5),
				},
			},
		},

		/////////////////// number operator
		{
			name:  "double nested string value",
			given: `{"person":{"age":{"$gte":10}}}`,
			want: M{
				"person.age": M{
					"$gte": float64(10),
				},
			},
		},

		/////////////////// array operator
		{
			name:  "double nested string value",
			given: `{"person":{"age":{"$in":[10, 20]}}}`,
			want: M{
				"person.age": M{
					"$in": []interface{}{float64(10), float64(20)},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var given M
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
		given M
		want  M
	}{
		/////////////////// string operator
		{
			name: "nested string value",
			given: M{
				"name": M{
					"$eq": "raymond",
				},
			},
			want: M{
				"name": M{
					"$eq": "raymond",
				},
			},
		},
		{
			name: "nested string value",
			given: M{
				"filter": M{
					"person": M{
						"age": M{
							"$eq": float64(5),
						},
					},
				},
			},
			want: M{
				"filter.person.age": M{
					"$eq": float64(5),
				},
			},
		},

		/////////////////// number operator
		{
			name: "double nested string value",
			given: M{
				"person": M{
					"age": M{
						"$gte": float64(10),
					},
				},
			},
			want: M{
				"person.age": M{
					"$gte": float64(10),
				},
			},
		},

		/////////////////// array operator
		{
			name: "double nested string value",
			given: M{
				"person": M{
					"age": M{
						"$in": []interface{}{float64(10), float64(20)},
					},
				},
			},
			want: M{
				"person.age": M{
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
		given   M
		want    M
		wantErr bool
		err     error
	}{
		/////////////////// $or operator
		{
			name: "top level $or operator - nothing to flatten",
			given: M{
				"$or": []M{
					{
						"age": M{
							"$in": []int{10, 11, 12},
						},
					},
					{
						"temperatures": 39.9,
					},
				},
			},
			want: M{
				"$or": []M{
					{
						"age": M{
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
			given: M{
				"$or": []M{
					{
						"person": M{
							"age": M{
								"$in": []int{10, 11, 12},
							},
						},
					},
					{
						"places": M{
							"temperatures": 39.9,
						},
					},
				},
			},
			want: M{
				"$or": []M{
					{
						"person.age": M{
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
			given: M{
				"$or": []M{
					{
						"person": M{
							"age": M{
								"$in": []int{10, 11, 12},
							},
						},
					},
					{
						"places": M{
							"temperatures": 39.9,
						},
					},
				},
			},
			want: M{
				"$or": []M{
					{
						"person.age": M{
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
			given: M{
				"$or": M{
					"person": M{
						"age": M{
							"$in": []int{10, 11, 12},
						},
					},
					"places": M{
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
			given: M{
				"$and": []M{
					{
						"age": M{
							"$in": []int{10, 11, 12},
						},
					},
					{
						"temperatures": 39.9,
					},
				},
			},
			want: M{
				"$and": []M{
					{
						"age": M{
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
			given: M{
				"$and": []M{
					{
						"person": M{
							"age": M{
								"$in": []int{10, 11, 12},
							},
						},
					},
					{
						"places": M{
							"temperatures": 39.9,
						},
					},
				},
			},
			want: M{
				"$and": []M{
					{
						"person.age": M{
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
			given: M{
				"$and": []M{
					{
						"person": M{
							"age": M{
								"$in": []int{10, 11, 12},
							},
						},
					},
					{
						"places": M{
							"temperatures": 39.9,
						},
					},
				},
			},
			want: M{
				"$and": []M{
					{
						"person.age": M{
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
			given: M{
				"$and": M{
					"person": M{
						"age": M{
							"$in": []int{10, 11, 12},
						},
					},
					"places": M{
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
			given: M{
				"$and": []M{
					{
						"$or": []M{
							{
								"age": M{
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
			want: M{
				"$and": []M{
					{
						"$or": []M{
							{
								"age": M{
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
			given: M{
				"$and": []M{
					{
						"$or": []M{
							{
								"person": M{
									"age": M{
										"$in": []int{10, 11, 12},
									},
								},
							},
							{
								"places": M{
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
			want: M{
				"$and": []M{
					{
						"$or": []M{
							{
								"person.age": M{
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
				if !errors.Is(err, test.err) {
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
			want: M{
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
			want: M{
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
			want: M{
				"$and": []interface{}{
					M{
						"age": M{
							"$gte": float64(10),
						},
					},
					M{
						"$or": []interface{}{
							M{
								"type": "weekly",
							},
							M{
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
			want: M{
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
	err := json.Unmarshal(ghEvent, &given)
	if err != nil {
		t.Errorf("failed to unmarshal JSON: %v", err)
	}

	err = json.Unmarshal(ghEventFlat, &want)
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

func BenchmarkFlattenArray(b *testing.B) {
	test := `
        [
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
        ]`

	want := M{
		"0.hallo.lorem":             []interface{}{float64(20), float64(2)},
		"0.hallo.ipsum.dolor":       []interface{}{"1", "10"},
		"1.game.authors.first_name": "George",
		"1.game.authors.last_name":  "Martin",
		"1.game.name":               "Elden Ring",
		"2.person.ages":             []interface{}{float64(100), float64(-1)},
		"2.person.names.parts":      []interface{}{"ray", "mond"},
	}

	var given interface{}
	err := json.Unmarshal([]byte(test), &given)
	if err != nil {
		require.NoError(b, err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		got, err := Flatten(given)
		require.NoError(b, err)

		if !jsonEqual(got, want) {
			b.Errorf("mismatch:\ngot:  %+v\nwant: %+v", got, want)
		}
	}
}

func BenchmarkFlattenMap(b *testing.B) {
	test := M{
		"$and": []M{
			{
				"$or": []M{
					{
						"person": M{
							"age": M{
								"$in": []int{10, 11, 12},
							},
						},
					},
					{
						"places": M{
							"temperatures": 39.9,
						},
					},
				},
			},
			{
				"city": "lagos",
			},
		},
	}

	want := M{
		"$and": []M{
			{
				"$or": []M{
					{
						"person.age": M{
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
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		got, err := Flatten(test)
		require.NoError(b, err)

		if !jsonEqual(got, want) {
			b.Errorf("mismatch:\ngot:  %+v\nwant: %+v", got, want)
		}
	}
}

func BenchmarkFlattenLargeJson(b *testing.B) {
	var given, want interface{}
	err := json.Unmarshal(ghEvent, &given)
	if err != nil {
		b.Errorf("failed to unmarshal JSON: %v", err)
	}

	err = json.Unmarshal(ghEventFlat, &want)
	if err != nil {
		b.Errorf("failed to unmarshal JSON: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		got, err := Flatten(given)
		require.NoError(b, err)

		if !jsonEqual(got, want) {
			expectedJson, _ := json.MarshalIndent(got, "", " ")
			b.Errorf("%v\n", string(expectedJson))
		}
	}
}

func BenchmarkFlattenOperators(b *testing.B) {
	given := M{
		"filter": M{
			"person": M{
				"age": M{
					"$eq": float64(5),
				},
			},
		},
	}
	want := M{
		"filter.person.age": M{
			"$eq": float64(5),
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		got, err := Flatten(given)
		require.NoError(b, err)

		if !jsonEqual(got, want) {
			expectedJson, _ := json.MarshalIndent(got, "", " ")
			b.Errorf("%v\n", string(expectedJson))
		}
	}
}

func BenchmarkFlattenWithPrefix(b *testing.B) {
	test := `{
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
			}`

	want := M{
		"data.hello.lorem.ipsum": "again",
		"data.hello.lorem.dolor": "sit",
		"data.world.lorem.ipsum": "again",
		"data.world.lorem.dolor": "sit",
	}

	var given interface{}
	err := json.Unmarshal([]byte(test), &given)
	if err != nil {
		require.NoError(b, err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		got, err := FlattenWithPrefix("data", given)
		require.NoError(b, err)

		if !jsonEqual(got, want) {
			expectedJson, _ := json.MarshalIndent(got, "", " ")
			b.Errorf("%v\n", string(expectedJson))
		}
	}
}
