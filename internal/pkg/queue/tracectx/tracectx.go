// Package tracectx carries W3C trace context across the Asynq queue boundary.
//
// Asynq tasks are an opaque payload between producer and consumer. To preserve
// the trace span tree across an enqueue, we wrap the payload in a small
// envelope that holds a propagation carrier alongside the original bytes. The
// envelope is detected by a single magic byte prefix so legacy/in-flight
// payloads (without an envelope) continue to process during a rolling deploy.
package tracectx

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/frain-dev/convoy/queue"
)

// envelopeMagic is a single-byte prefix that flags a payload as carrying a
// trace-context envelope. Picked so it cannot start a valid JSON payload (which
// would begin with `{` or `[`) or a typical msgpack map (0x80–0x9f / 0xde /
// 0xdf), so we can confidently distinguish envelopes from legacy raw bytes.
const envelopeMagic byte = 0x01

type envelope struct {
	TraceContext map[string]string `json:"tc"`
	Payload      []byte            `json:"p"`
}

// InjectIntoJob writes the W3C trace context from ctx into job.Headers so the
// queue producer can serialise it into the task envelope. Safe to call when
// the context has no active span — Headers stays empty and the consumer just
// starts a root span.
func InjectIntoJob(ctx context.Context, job *queue.Job) {
	if job == nil {
		return
	}
	if job.Headers == nil {
		job.Headers = map[string]string{}
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(job.Headers))
}

// Wrap encodes a payload + headers into an envelope and returns the bytes that
// should be sent to the queue. On marshal failure the original payload is
// returned so we degrade rather than drop the message.
func Wrap(headers map[string]string, payload []byte) []byte {
	env := envelope{TraceContext: headers, Payload: payload}
	body, err := json.Marshal(env)
	if err != nil {
		return payload
	}
	out := make([]byte, 0, len(body)+1)
	out = append(out, envelopeMagic)
	out = append(out, body...)
	return out
}

// Unwrap inspects bytes pulled from the queue. If they carry an envelope it
// returns the inner payload and the trace headers. Otherwise (legacy / raw
// payloads enqueued before the producer started wrapping) it returns the bytes
// unchanged with a nil headers map.
func Unwrap(body []byte) (payload []byte, headers map[string]string) {
	if len(body) == 0 || body[0] != envelopeMagic {
		return body, nil
	}
	var env envelope
	if err := json.Unmarshal(body[1:], &env); err != nil {
		return body, nil
	}
	return env.Payload, env.TraceContext
}

// ExtractContext returns a context with the trace context decoded from
// headers attached. ctx is the consumer's base context (typically the asynq
// handler's). When headers is empty the original ctx is returned.
func ExtractContext(ctx context.Context, headers map[string]string) context.Context {
	if len(headers) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(headers))
}
