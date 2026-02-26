package msgpack

import (
	"bytes"

	"sync"

	"github.com/vmihailenco/msgpack/v5"
)

var (
	encPool = sync.Pool{
		New: func() any {
			var buf bytes.Buffer
			enc := msgpack.NewEncoder(&buf)
			enc.SetCustomStructTag("json")
			return enc
		},
	}

	decPool = sync.Pool{
		New: func() any {
			dec := msgpack.NewDecoder(nil)
			dec.SetCustomStructTag("json")
			return dec
		},
	}
)

func EncodeMsgPack(payload interface{}) ([]byte, error) {
	enc := encPool.Get().(*msgpack.Encoder)
	defer encPool.Put(enc)

	enc.SetCustomStructTag("json")

	buf := enc.Writer().(*bytes.Buffer)
	buf.Reset()

	err := enc.Encode(payload)
	if err != nil {
		return nil, err
	}

	res := make([]byte, buf.Len())
	copy(res, buf.Bytes())
	return res, nil
}

func DecodeMsgPack(pack []byte, target interface{}) error {
	dec := decPool.Get().(*msgpack.Decoder)
	defer decPool.Put(dec)

	dec.Reset(bytes.NewReader(pack))
	dec.SetCustomStructTag("json")

	err := dec.Decode(target)
	if err != nil {
		return err
	}

	return nil
}
