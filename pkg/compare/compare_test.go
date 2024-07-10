package compare

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/nsf/jsondiff"
)

func TestGenCombos(t *testing.T) {
	f, err := flatten.Flatten(comboTestPayload)
	require.NoError(t, err)

	combos, err := genCombos(f, "data.mentioned_profiles.$.fid")
	require.NoError(t, err)

	require.Len(t, combos, 6)
}

func TestNestedPositionalArrayFilter(t *testing.T) {
	p, err := flatten.Flatten(comboTestPayload)
	if err != nil {
		t.Errorf("failed to flatten JSON: %v", err)
	}

	f, err := flatten.Flatten(comboTestFilter)
	if err != nil {
		t.Errorf("failed to flatten JSON: %v", err)
	}

	matched, err := Compare(p, f)
	if err != nil {
		t.Error(err)
	}
	require.True(t, matched)
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name    string
		payload interface{}
		filter  map[string]interface{}
		want    bool
	}{
		{
			name: "regex",
			payload: map[string]interface{}{
				"event": "qwerty",
			},
			filter: map[string]interface{}{
				"event": map[string]interface{}{
					"$regex": "^[a-zA-Z]+$",
				},
			},
			want: true,
		},
		{
			name: "regex - with prefix",
			payload: map[string]interface{}{
				"event": "cs_qwerty",
			},
			filter: map[string]interface{}{
				"event": map[string]interface{}{
					"$regex": "^cs_[a-zA-Z]+$",
				},
			},
			want: true,
		},
		{
			name: "regex - overly complex example",
			payload: map[string]interface{}{
				"event": "https://admin:admin@mfs-registry-stg.g4.app.cloud.comcast.net/eureka/apps/MFSAGENT/mfsagent:e1432431e46cf610d06e2dbcda13b069?status=UP&lastDirtyTimestamp=1643797857108",
			},
			filter: map[string]interface{}{
				"event": map[string]interface{}{
					"$regex": "^(?P<scheme>[^:\\/?#]+):(?:\\/\\/)?(?:(?:(?P<login>[^:]+)(?::(?P<password>[^@]+)?)?@)?(?P<host>[^@\\/?#:]*)(?::(?P<port>\\d+)?)?)?(?P<path>[^?#]*)(?:\\?(?P<query>[^#]*))?(?:#(?P<fragment>.*))?",
				},
			},
			want: true,
		},
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
			name: "$and",
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
						"person.age": map[string]interface{}{
							"$gte": 10,
						},
					},
					map[string]interface{}{
						"type": "weekly",
					},
					map[string]interface{}{
						"cities": "lagos",
					},
				},
			},
			want: true,
		},
		{
			name: "$or",
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
				"$or": []interface{}{
					map[string]interface{}{
						"type": "monthly",
					},
					map[string]interface{}{
						"cities": "lagos",
					},
				},
			},
			want: true,
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
						"person.age": map[string]interface{}{
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
		{
			name:    "Nothing",
			payload: map[string]interface{}{},
			filter:  map[string]interface{}{},
			want:    true,
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

			matched, err := Compare(p, f)
			if err != nil {
				t.Error(err)
			}
			if !jsonEqual(matched, tt.want) {
				t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", matched, tt.want)
			}
		})
	}
}

func TestCompareEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		payload interface{}
		filter  map[string]interface{}
		want    bool
		err     error
	}{
		{
			name: "trailing array operator (.$) - 1",
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
				"data.$.venue.$": "test",
			},
			err: ErrTrailingDollarOpNotAllowed,
		},
		{
			name: "trailing array operator (.$) - 2",
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
				"$.venues.$.lagos.$": "ikeja",
			},
			err: ErrTrailingDollarOpNotAllowed,
		},
		{
			name: "array operator (.$) - 3",
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
				"$.venues.$.lagos.$": "bariga",
			},
			err: ErrTrailingDollarOpNotAllowed,
		},
		{
			name: "array operator (.$) - 4",
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
				"$.": "bariga",
			},
			want: false,
		},
		{
			name: "array operator (.$) - 5",
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
				".$": "bariga",
			},
			err: ErrTrailingDollarOpNotAllowed,
		},
		{
			name: "weird case",
			payload: []interface{}{
				map[string]interface{}{
					"event": "meetup",
				},
				map[string]interface{}{
					"place": "bariga",
				},
			},
			filter: map[string]interface{}{
				"$.$": "test",
			},
			err: ErrTrailingDollarOpNotAllowed,
		},
		{
			name: "weird case",
			payload: []interface{}{
				map[string]interface{}{
					"event": "meetup",
				},
				map[string]interface{}{
					"place": "bariga",
				},
			},
			filter: map[string]interface{}{
				"$..$": "test",
			},
			err: ErrTrailingDollarOpNotAllowed,
		},
		{
			name:    "weird case",
			payload: map[string]interface{}{},
			filter: map[string]interface{}{
				"key": "value",
			},
			want: false,
		},
		{
			name:    "weird case",
			payload: []interface{}{},
			filter: map[string]interface{}{
				"key": "value",
			},
			want: false,
		},
		{
			name: "weird case",
			payload: []interface{}{
				map[string]interface{}{
					"event": "meetup",
				},
				map[string]interface{}{
					"place": "bariga",
				},
			},
			filter: map[string]interface{}{
				"a.$.b.$.c.$.d.$.e": "test",
			},
			err: fmt.Errorf("too many segments, expected at most 3 but got 4"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := flatten.Flatten(tt.payload)
			if err != nil {
				t.Errorf("failed to flatten JSON: %v", err)
				return
			}

			f, err := flatten.Flatten(tt.filter)
			if err != nil {
				t.Errorf("failed to flatten JSON: %v", err)
				return
			}

			matched, err := Compare(p, f)
			if tt.err != nil {
				if tt.err.Error() != err.Error() {
					t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", err.Error(), tt.err.Error())
				}
				return
			}

			if !jsonEqual(matched, tt.want) {
				t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", matched, tt.want)
			}
		})
	}
}

func TestCompareEdgeCasesWithOperators(t *testing.T) {
	tests := []struct {
		name    string
		payload interface{}
		filter  map[string]interface{}
		want    bool
		err     error
	}{
		{
			name: "weird case",
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
								"ifako",
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
				"$or": []interface{}{
					map[string]interface{}{
						"$.venues.$.lagos": "ifako",
					},
					map[string]interface{}{
						"$.venues.$.ibadan": "dugbe",
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := flatten.Flatten(tt.payload)
			if err != nil {
				t.Errorf("failed to flatten JSON: %v", err)
				return
			}

			f, err := flatten.Flatten(tt.filter)
			if err != nil {
				t.Errorf("failed to flatten JSON: %v", err)
				return
			}

			matched, err := Compare(p, f)
			if tt.err != nil {
				if tt.err.Error() != err.Error() {
					fmt.Printf("f: %v\n", f)
					t.Errorf("mismatch:\ngot:  %+v\nwant: %+v", err.Error(), tt.err.Error())
				}
				return
			}

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

func BenchmarkCompareNestedArrayOperator(b *testing.B) {
	payload := map[string]interface{}{
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
	}

	filter := map[string]interface{}{
		"data.$.speakers.$.name": "raymond",
		"swag":                   "hoodies",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		p, err := flatten.Flatten(payload)
		require.NoError(b, err)

		f, err := flatten.Flatten(filter)
		require.NoError(b, err)

		matched, err := Compare(p, f)
		require.NoError(b, err)

		if !matched {
			b.Errorf("mismatch:\ngot:  %+v\nwant: %+v", matched, true)
		}
	}
}

func BenchmarkCompareAndOr(b *testing.B) {
	payload := map[string]interface{}{
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
	}
	filter := map[string]interface{}{
		"$and": []interface{}{
			map[string]interface{}{
				"person.age": map[string]interface{}{
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
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		p, err := flatten.Flatten(payload)
		require.NoError(b, err)

		f, err := flatten.Flatten(filter)
		require.NoError(b, err)

		matched, err := Compare(p, f)
		require.NoError(b, err)

		if !matched {
			b.Errorf("mismatch:\ngot:  %+v\nwant: %+v", matched, true)
		}
	}
}

func BenchmarkCompareRegex(b *testing.B) {
	payload := map[string]interface{}{
		"event": "https://admin:admin@mfs-registry-stg.g4.app.cloud.comcast.net/eureka/apps/MFSAGENT/mfsagent:e1432431e46cf610d06e2dbcda13b069?status=UP&lastDirtyTimestamp=1643797857108",
	}

	filter := map[string]interface{}{
		"event": map[string]interface{}{
			"$regex": "^(?P<scheme>[^:\\/?#]+):(?:\\/\\/)?(?:(?:(?P<login>[^:]+)(?::(?P<password>[^@]+)?)?@)?(?P<host>[^@\\/?#:]*)(?::(?P<port>\\d+)?)?)?(?P<path>[^?#]*)(?:\\?(?P<query>[^#]*))?(?:#(?P<fragment>.*))?",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		p, err := flatten.Flatten(payload)
		require.NoError(b, err)

		f, err := flatten.Flatten(filter)
		require.NoError(b, err)

		matched, err := Compare(p, f)
		require.NoError(b, err)

		if !matched {
			b.Errorf("mismatch:\ngot:  %+v\nwant: %+v", matched, true)
		}
	}
}

var comboTestPayload = map[string]interface{}{
	"created_at": 1720530480,
	"type":       "cast.created",
	"data": map[string]interface{}{
		"object":          "cast",
		"hash":            "0xdc7236eb81905f125dcf629a07abfb0b5f8bcd48",
		"thread_hash":     "0xdc7236eb81905f125dcf629a07abfb0b5f8bcd48",
		"parent_hash":     nil,
		"parent_url":      nil,
		"root_parent_url": nil,
		"parent_author": map[string]interface{}{
			"fid": nil,
		},
		"author": map[string]interface{}{
			"object":          "user",
			"fid":             19960,
			"custody_address": "0xd1b702203b1b3b641a699997746bd4a12d157909",
			"username":        "shreyas-chorge",
			"display_name":    "Shreyas",
			"pfp_url":         "https://i.imgur.com/LPzRlQl.jpg",
			"profile": map[string]interface{}{
				"bio": map[string]interface{}{
					"text": "Everyday regular normal guy | 👨‍💻 @neynar ...",
				},
			},
			"follower_count":  192,
			"following_count": 115,
			"verifications": []interface{}{
				"0xd1b702203b1b3b641a699997746bd4a12d157909",
				"0x7ea5dada4021c2c625e73d2a78882e91b93c174c",
			},
			"verified_addresses": map[string]interface{}{
				"eth_addresses": []interface{}{
					"0xd1b702203b1b3b641a699997746bd4a12d157909",
					"0x7ea5dada4021c2c625e73d2a78882e91b93c174c",
				},
				"sol_addresses": []interface{}{},
			},
			"active_status": "inactive",
			"power_badge":   false,
		},
		"text":      "@rishav @dylsteck.eth @antimofm.eth @kevinoconnell",
		"timestamp": "2024-07-09T05:55:21.000Z",
		"embeds":    []interface{}{},
		"reactions": map[string]interface{}{
			"likes_count":   0,
			"recasts_count": 0,
			"likes":         []interface{}{},
			"recasts":       []interface{}{},
		},
		"replies": map[string]interface{}{
			"count": 0,
		},
		"channel": nil,
		"mentioned_profiles": []interface{}{
			map[string]interface{}{
				"object":          "user",
				"fid":             12,
				"custody_address": "0x7355b6af053e5d0fdcbc23cc8a45b0cd85034378",
				"username":        "rishav",
				"display_name":    "rtest",
				"pfp_url":         "https://i.imgur.com/j1phftZ.jpg",
				"profile": map[string]interface{}{
					"bio": map[string]interface{}{
						"text":               "rtest",
						"mentioned_profiles": []interface{}{},
					},
				},
				"follower_count":  4,
				"following_count": 50,
				"verifications":   []interface{}{},
				"verified_addresses": map[string]interface{}{
					"eth_addresses": []interface{}{},
					"sol_addresses": []interface{}{},
				},
				"active_status": "inactive",
				"power_badge":   false,
			},
			map[string]interface{}{
				"object":          "user",
				"fid":             123,
				"custody_address": "0x5e79f690ccd42007d5a0ad678cd47474339400e3",
				"username":        "dylsteck.eth",
				"display_name":    "dylan",
				"pfp_url":         "https://i.imgur.com/2UTZYvn.png",
				"profile": map[string]interface{}{
					"bio": map[string]interface{}{
						"text":               "building products /neynar, hacking /farhack, yapping /dylan | dylansteck.com",
						"mentioned_profiles": []interface{}{},
					},
				},
				"follower_count":  72663,
				"following_count": 1280,
				"verifications": []interface{}{
					"0x7e37c3a9349227b60503ddb1574a76d10c6bc48e",
				},
				"verified_addresses": map[string]interface{}{
					"eth_addresses": []interface{}{
						"0x7e37c3a9349227b60503ddb1574a76d10c6bc48e",
					},
					"sol_addresses": []interface{}{
						"CYzdpr7xtH3SBf81tpdRsPyhZqv4s6BbkwHzHYkc6FDr",
					},
				},
				"active_status": "inactive",
				"power_badge":   true,
			},
			map[string]interface{}{
				"object":          "user",
				"fid":             112,
				"custody_address": "0xb6452061188bf3f456aabfa46b648773779e6961",
				"username":        "antimofm.eth",
				"display_name":    "antimo 🎩",
				"pfp_url":         "https://i.imgur.com/t4LDaI8.jpg",
				"profile": map[string]interface{}{
					"bio": map[string]interface{}{
						"text":               "/red designer /design host /condensed author /gang leader /nfs maxi /hyperclient founder",
						"mentioned_profiles": []interface{}{},
					},
				},
				"follower_count":  103282,
				"following_count": 1064,
				"verifications": []interface{}{
					"0xfa922ce609fed47950e3f48662c9651d42ada194",
				},
				"verified_addresses": map[string]interface{}{
					"eth_addresses": []interface{}{
						"0xfa922ce609fed47950e3f48662c9651d42ada194",
					},
					"sol_addresses": []interface{}{},
				},
				"active_status": "inactive",
				"power_badge":   true,
			},
			map[string]interface{}{
				"object":          "user",
				"fid":             12334,
				"custody_address": "0x4622146b77ecefe4ca7552a81949d54eac991512",
				"username":        "kevinoconnell",
				"display_name":    "kevin",
				"pfp_url":         "https://imagedelivery.net/BXluQx4ige9GuW0Ia56BHw/132d2b90-59f1-4624-a6d3-433f879ecd00/rectcrop3",
				"profile": map[string]interface{}{
					"bio": map[string]interface{}{
						"text":               "I like building things and exploring new places. MrKevinOConnell.github @neynar prev @hypeshot i ask questions in /braindump",
						"mentioned_profiles": []interface{}{},
					},
				},
				"follower_count":  46543,
				"following_count": 2465,
				"verifications": []interface{}{
					"0xedd3783e8c7c52b80cfbd026a63c207edc9cbee7",
					"0x69689f02c4154b049fb42761ef8fa00808f1b7ea",
				},
				"verified_addresses": map[string]interface{}{
					"eth_addresses": []interface{}{
						"0xedd3783e8c7c52b80cfbd026a63c207edc9cbee7",
						"0x69689f02c4154b049fb42761ef8fa00808f1b7ea",
					},
					"sol_addresses": []interface{}{},
				},
				"active_status": "inactive",
				"power_badge":   true,
			},
			map[string]interface{}{
				"object": "user",
				"fid":    1232323,
			},
			map[string]interface{}{
				"object": "user",
				"fid":    1,
			},
		},
	},
}

var comboTestFilter = map[string]interface{}{
	"$or": []interface{}{
		map[string]interface{}{
			"$and": []interface{}{
				map[string]interface{}{
					"type": "user.updated",
				},
				map[string]interface{}{
					"data": map[string]interface{}{
						"fid": map[string]interface{}{
							"$in": []interface{}{},
						},
					},
				},
			},
		},
		map[string]interface{}{
			"$and": []interface{}{
				map[string]interface{}{
					"type": "cast.created",
				},
				map[string]interface{}{
					"$or": []interface{}{
						map[string]interface{}{
							"data": map[string]interface{}{
								"author": map[string]interface{}{
									"fid": map[string]interface{}{
										"$in": []interface{}{},
									},
								},
							},
						},
						map[string]interface{}{
							"data": map[string]interface{}{
								"parent_author": map[string]interface{}{
									"fid": map[string]interface{}{
										"$in": []interface{}{},
									},
								},
							},
						},
						map[string]interface{}{
							"data": map[string]interface{}{
								"mentioned_profiles.$": map[string]interface{}{
									"fid": map[string]interface{}{
										"$in": []interface{}{
											1232323,
										},
									},
								},
							},
						},
						map[string]interface{}{
							"data": map[string]interface{}{
								"parent_url": map[string]interface{}{
									"$in": []interface{}{},
								},
							},
						},
						map[string]interface{}{
							"data": map[string]interface{}{
								"root_parent_url": map[string]interface{}{
									"$in": []interface{}{},
								},
							},
						},
					},
				},
			},
		},
		map[string]interface{}{
			"$and": []interface{}{
				map[string]interface{}{
					"type": "follow.created",
				},
				map[string]interface{}{
					"$or": []interface{}{
						map[string]interface{}{
							"data": map[string]interface{}{
								"user": map[string]interface{}{
									"fid": map[string]interface{}{
										"$in": []interface{}{},
									},
								},
							},
						},
						map[string]interface{}{
							"data": map[string]interface{}{
								"target_user": map[string]interface{}{
									"fid": map[string]interface{}{
										"$in": []interface{}{},
									},
								},
							},
						},
					},
				},
			},
		},
		map[string]interface{}{
			"$and": []interface{}{
				map[string]interface{}{
					"type": "follow.deleted",
				},
				map[string]interface{}{
					"$or": []interface{}{
						map[string]interface{}{
							"data": map[string]interface{}{
								"user": map[string]interface{}{
									"fid": map[string]interface{}{
										"$in": []interface{}{},
									},
								},
							},
						},
						map[string]interface{}{
							"data": map[string]interface{}{
								"target_user": map[string]interface{}{
									"fid": map[string]interface{}{
										"$in": []interface{}{},
									},
								},
							},
						},
					},
				},
			},
		},
		map[string]interface{}{
			"$and": []interface{}{
				map[string]interface{}{
					"type": "reaction.created",
				},
				map[string]interface{}{
					"$or": []interface{}{
						map[string]interface{}{
							"data": map[string]interface{}{
								"user": map[string]interface{}{
									"fid": map[string]interface{}{
										"$in": []interface{}{},
									},
								},
							},
						},
						map[string]interface{}{
							"data": map[string]interface{}{
								"cast": map[string]interface{}{
									"author": map[string]interface{}{
										"fid": map[string]interface{}{
											"$in": []interface{}{},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		map[string]interface{}{
			"$and": []interface{}{
				map[string]interface{}{
					"type": "reaction.deleted",
				},
				map[string]interface{}{
					"$or": []interface{}{
						map[string]interface{}{
							"data": map[string]interface{}{
								"user": map[string]interface{}{
									"fid": map[string]interface{}{
										"$in": []interface{}{},
									},
								},
							},
						},
						map[string]interface{}{
							"data": map[string]interface{}{
								"cast": map[string]interface{}{
									"author": map[string]interface{}{
										"fid": map[string]interface{}{
											"$in": []interface{}{},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	},
}
