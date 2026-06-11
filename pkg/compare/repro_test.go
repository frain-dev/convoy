package compare

import "testing"

// TestInMixedTypeArrayNoPanic ensures the $in operator does not panic when the
// payload array field holds mixed JSON types (e.g. a string and a number).
// Arbitrary webhook payloads can produce such arrays, and previously the sort
// comparator in `in` did an unchecked type assertion on pCopy[j], panicking.
func TestInMixedTypeArrayNoPanic(t *testing.T) {
	payload := map[string]interface{}{
		"ids": []interface{}{"a", float64(1)},
	}
	filter := map[string]interface{}{
		"ids": map[string]interface{}{"$in": "a"},
	}

	got, err := Compare(payload, filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Fatalf("expected match for \"a\" in mixed array, got false")
	}
}
