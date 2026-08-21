package event_deliveries

import "testing"

func TestEventDeliveriesPagedInnerDesc(t *testing.T) {
	tests := []struct {
		sortOrder, direction string
		wantDesc             bool
	}{
		{sortOrder: "DESC", direction: "next", wantDesc: true},
		{sortOrder: "ASC", direction: "prev", wantDesc: true},
		{sortOrder: "ASC", direction: "next", wantDesc: false},
		{sortOrder: "DESC", direction: "prev", wantDesc: false},
	}
	for _, tc := range tests {
		if got := eventDeliveriesPagedInnerDesc(tc.sortOrder, tc.direction); got != tc.wantDesc {
			t.Fatalf("sort=%s direction=%s: got %v want %v", tc.sortOrder, tc.direction, got, tc.wantDesc)
		}
	}
}
