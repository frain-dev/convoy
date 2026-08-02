package api

import (
	"bytes"
	"io"
	"net/http/httptest"
	"sync"
	"testing"
)

var benchmarkPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 16*1024))
	},
}

func BenchmarkIngestion(b *testing.B) {
	// A typical 5KB webhook payload
	payload := bytes.Repeat([]byte("a"), 5*1024)
	maxIngestSize := int64(1024 * 1024) // 1MB limit

	b.Run("Before_io.ReadAll", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
			raw, err := io.ReadAll(io.LimitReader(r.Body, maxIngestSize))
			if err != nil {
				b.Fatal(err)
			}
			_ = raw
		}
	})

	b.Run("After_sync.Pool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
			buf := benchmarkPool.Get().(*bytes.Buffer)
			buf.Reset()
			
			_, err := io.Copy(buf, io.LimitReader(r.Body, maxIngestSize))
			if err != nil {
				b.Fatal(err)
			}
			raw := buf.Bytes()
			
			// Simulate processing, then return to pool
			if buf.Cap() <= 64*1024 {
				benchmarkPool.Put(buf)
			}
			_ = raw
		}
	})
}
