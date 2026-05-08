package worker

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Pre-Epic-10 envelope: a single magic-byte prefix followed by a JSON object
// with "tc" (trace context) and "p" (raw payload bytes). Tasks enqueued
// before Epic 10 ride this format; tasks enqueued after ride asynq.Task
// Headers natively. The transitional helper detects the magic byte and
// returns the inner payload + headers so handlers continue working.
func TestTryUnwrapLegacyEnvelope_RoundtripsEnvelope(t *testing.T) {
	innerPayload := []byte(`{"event":"x"}`)
	headers := map[string]string{"traceparent": "00-abc-def-01"}

	body, err := json.Marshal(struct {
		TC map[string]string `json:"tc"`
		P  []byte            `json:"p"`
	}{TC: headers, P: innerPayload})
	require.NoError(t, err)
	wire := append([]byte{legacyEnvelopeMagic}, body...)

	env := tryUnwrapLegacyEnvelope(wire)
	require.NotNil(t, env, "envelope-prefixed bytes must be recognised")
	require.Equal(t, innerPayload, env.payload)
	require.Equal(t, headers, env.headers)
}

// Native-headers payloads (Epic 10) and any unrelated bytes that don't start
// with the legacy magic byte must fall through unchanged.
func TestTryUnwrapLegacyEnvelope_RawPayloadIsPassThrough(t *testing.T) {
	for _, name := range []string{"empty", "json-object", "msgpack-fixmap"} {
		t.Run(name, func(t *testing.T) {
			var raw []byte
			switch name {
			case "empty":
				raw = nil
			case "json-object":
				raw = []byte(`{"event":"x"}`)
			case "msgpack-fixmap":
				raw = []byte{0x81, 0xa1, 0x61, 0x01} // {"a": 1}
			}
			require.Nil(t, tryUnwrapLegacyEnvelope(raw))
		})
	}
}

// A magic-byte prefix followed by garbage (not parseable as our envelope
// JSON) returns nil so the consumer falls through to the native-headers
// path; the raw bytes flow to the handler which fails-loudly via its own
// codec.
func TestTryUnwrapLegacyEnvelope_MalformedReturnsNil(t *testing.T) {
	bad := append([]byte{legacyEnvelopeMagic}, []byte("not-json")...)
	require.Nil(t, tryUnwrapLegacyEnvelope(bad))
}
