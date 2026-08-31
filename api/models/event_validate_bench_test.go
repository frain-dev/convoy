package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func benchPayload(size int) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"name":%q}`, strings.Repeat("a", size)))
}

func BenchmarkCreateEventValidate(b *testing.B) {
	for _, size := range []int{0, 100, 1_000, 10_000} {
		b.Run(fmt.Sprintf("payload_%d", size), func(b *testing.B) {
			e := &CreateEvent{
				EndpointID: "endpoint-1",
				EventType:  "invoice.paid",
				Data:       benchPayload(size),
			}

			for b.Loop() {
				if err := e.Validate(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
