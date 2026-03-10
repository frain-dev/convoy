package msgpack

import (
	"bytes"
	"sync"

	"github.com/vmihailenco/msgpack/v5"
)

// bufferPool is a pool of reusable buffers to reduce GC pressure
// Pre-allocates 1MB buffers suitable for large event payloads
var bufferPool = sync.Pool{
	New: func() interface{} {
		buf := new(bytes.Buffer)
		buf.Grow(1024 * 1024) // Pre-allocate 1MB for large payloads
		return buf
	},
}

func EncodeMsgPack(payload interface{}) ([]byte, error) {
	// Get buffer from pool
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset() // Clear previous contents
	defer bufferPool.Put(buf)

	enc := msgpack.NewEncoder(buf)
	enc.SetCustomStructTag("json")

	err := enc.Encode(payload)
	if err != nil {
		return nil, err
	}

	// Must make a copy since we're returning buffer to pool
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

func DecodeMsgPack(pack []byte, target interface{}) error {
	var buf bytes.Buffer
	buf.Write(pack)

	enc := msgpack.NewDecoder(&buf)
	enc.SetCustomStructTag("json")

	err := enc.Decode(&target)
	if err != nil {
		return err
	}

	return nil
}
