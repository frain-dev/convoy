package compare

import (
	"testing"

	"github.com/frain-dev/convoy/pkg/flatten"
)

func TestFilterMissingFieldsAndInvalidOperands(t *testing.T) {
	tests := []struct {
		name            string
		payload, filter flatten.M
		want, wantError bool
	}{
		{"missing required field",
			flatten.M{"type": "invoice"},
			flatten.M{"type": "invoice", "customer": "alice"},
			false,
			false},
		{"missing exists false",
			flatten.M{"type": "invoice"},
			flatten.M{"customer": flatten.M{"$exist": false}},
			true,
			false},
		{"missing exists true with match",
			flatten.M{"type": "invoice"},
			flatten.M{"type": "invoice", "customer": flatten.M{"$exist": true}},
			false,
			false},
		{"existing exists false",
			flatten.M{"customer": "alice"},
			flatten.M{"customer": flatten.M{"$exist": false}},
			false,
			false},
		{"invalid existing exists",
			flatten.M{"customer": "alice"},
			flatten.M{"customer": flatten.M{"$exist": "yes"}},
			false,
			true},
		{"invalid missing exists",
			flatten.M{"type": "invoice"},
			flatten.M{"customer": flatten.M{"$exist": "yes"}},
			false,
			true},
		{"nonnumeric less than",
			flatten.M{"amount": "invalid"},
			flatten.M{"amount": flatten.M{"$lt": 10.0}},
			false,
			true},
		{"invalid numeric operand",
			flatten.M{"amount": 5.0},
			flatten.M{"amount": flatten.M{"$lt": "invalid"}},
			false,
			true},
		{"valid less than",
			flatten.M{"amount": 5.0},
			flatten.M{"amount": flatten.M{"$lt": 10.0}},
			true,
			false},
		{"equal is not less than",
			flatten.M{"amount": 10.0},
			flatten.M{"amount": flatten.M{"$lt": 10.0}},
			false,
			false},
		{"missing empty object with match",
			flatten.M{"type": "invoice"},
			flatten.M{"type": "invoice", "customer": flatten.M{}},
			false,
			false},
		{"missing within or",
			flatten.M{"type": "invoice"},
			flatten.M{"$or": []interface{}{flatten.M{"type": "invoice", "customer": "alice"}, flatten.M{"type": "payment"}}},
			false,
			false},
		{"valid or branch",
			flatten.M{"type": "invoice"},
			flatten.M{"$or": []interface{}{flatten.M{"customer": "alice"}, flatten.M{"type": "invoice"}}},
			true,
			false},
		{"exists false plus comparison",
			flatten.M{"type": "invoice"},
			flatten.M{"customer": flatten.M{"$exist": false, "$eq": "alice"}},
			false,
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := flatten.Flatten(tt.payload)
			if err != nil {
				t.Fatal(err)
			}
			f, err := flatten.Flatten(tt.filter)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Compare(p, f)
			if got != tt.want || (err != nil) != tt.wantError {
				t.Fatalf("got (%v,%v), want match=%v error=%v", got, err, tt.want, tt.wantError)
			}
		})
	}
}
