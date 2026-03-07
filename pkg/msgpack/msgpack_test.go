package msgpack

import (
	"bytes"
	"sync"
	"testing"
)

// TestEncodeMsgPack verifies basic msgpack encoding functionality
func TestEncodeMsgPack(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name    string
		payload interface{}
		wantErr bool
	}{
		{
			name: "encode simple struct",
			payload: testStruct{
				Name:  "test",
				Value: 42,
			},
			wantErr: false,
		},
		{
			name: "encode map",
			payload: map[string]interface{}{
				"key1": "value1",
				"key2": 123,
			},
			wantErr: false,
		},
		{
			name:    "encode string",
			payload: "test string",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EncodeMsgPack(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeMsgPack() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(result) == 0 {
				t.Error("EncodeMsgPack() returned empty byte slice")
			}
		})
	}
}

// TestDecodeMsgPack verifies msgpack decoding functionality
func TestDecodeMsgPack(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := testStruct{
		Name:  "test",
		Value: 42,
	}

	// Encode
	encoded, err := EncodeMsgPack(original)
	if err != nil {
		t.Fatalf("EncodeMsgPack() failed: %v", err)
	}

	// Decode
	var decoded testStruct
	err = DecodeMsgPack(encoded, &decoded)
	if err != nil {
		t.Fatalf("DecodeMsgPack() failed: %v", err)
	}

	// Verify
	if decoded.Name != original.Name || decoded.Value != original.Value {
		t.Errorf("Decoded struct doesn't match original. Got %+v, want %+v", decoded, original)
	}
}

// TestBufferPooling verifies that buffer pooling works correctly
func TestBufferPooling(t *testing.T) {
	type largePayload struct {
		Data string `json:"data"`
	}

	// Create a large payload (similar to production)
	largeData := largePayload{
		Data: string(bytes.Repeat([]byte("x"), 100*1024)), // 100KB
	}

	// Encode multiple times to test pool reuse
	for i := 0; i < 10; i++ {
		result, err := EncodeMsgPack(largeData)
		if err != nil {
			t.Fatalf("EncodeMsgPack() iteration %d failed: %v", i, err)
		}
		if len(result) == 0 {
			t.Errorf("EncodeMsgPack() iteration %d returned empty result", i)
		}
	}
}

// TestConcurrentEncoding verifies thread-safety of buffer pooling
func TestConcurrentEncoding(t *testing.T) {
	type payload struct {
		ID    int    `json:"id"`
		Value string `json:"value"`
	}

	var wg sync.WaitGroup
	concurrency := 50

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			p := payload{
				ID:    id,
				Value: "test value",
			}

			result, err := EncodeMsgPack(p)
			if err != nil {
				t.Errorf("EncodeMsgPack() failed for ID %d: %v", id, err)
				return
			}

			if len(result) == 0 {
				t.Errorf("EncodeMsgPack() returned empty result for ID %d", id)
			}
		}(i)
	}

	wg.Wait()
}

// BenchmarkEncodeMsgPack benchmarks encoding performance
func BenchmarkEncodeMsgPack(b *testing.B) {
	type payload struct {
		Data string `json:"data"`
	}

	p := payload{
		Data: string(bytes.Repeat([]byte("x"), 10*1024)), // 10KB payload
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EncodeMsgPack(p)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeMsgPackLarge benchmarks encoding with large payloads (similar to production)
func BenchmarkEncodeMsgPackLarge(b *testing.B) {
	type payload struct {
		Data string `json:"data"`
	}

	p := payload{
		Data: string(bytes.Repeat([]byte("x"), 629*1024)), // 629KB payload (production size)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EncodeMsgPack(p)
		if err != nil {
			b.Fatal(err)
		}
	}
}
