package server

import (
	"testing"
	"time"
)

func TestHTTPServerTimeouts(t *testing.T) {
	tests := []struct {
		readSeconds uint64
		ackSeconds  uint64
		wantRead    time.Duration
		wantWrite   time.Duration
	}{
		{readSeconds: 0, ackSeconds: 0, wantRead: 30 * time.Second, wantWrite: 45 * time.Second},
		{readSeconds: 60, ackSeconds: 30, wantRead: 60 * time.Second, wantWrite: 75 * time.Second},
		{readSeconds: 30, ackSeconds: 120, wantRead: 30 * time.Second, wantWrite: 135 * time.Second},
	}
	for _, tt := range tests {
		gotRead, gotWrite := httpServerTimeouts(tt.readSeconds, tt.ackSeconds)
		if gotRead != tt.wantRead || gotWrite != tt.wantWrite {
			t.Fatalf("httpServerTimeouts(%d, %d) = %s, %s; want %s, %s",
				tt.readSeconds, tt.ackSeconds, gotRead, gotWrite, tt.wantRead, tt.wantWrite)
		}
	}
}
