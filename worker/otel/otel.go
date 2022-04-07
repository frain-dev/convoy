package otel

import (
	"context"

	"github.com/frain-dev/convoy/tracer"
	"github.com/frain-dev/taskq/v3"
)

type OpenTelemetryHook struct {
	tr tracer.Tracer
}

var _ taskq.ConsumerHook = (*OpenTelemetryHook)(nil)

func NewOtelHook(tr tracer.Tracer) taskq.ConsumerHook {
	otelHook := &OpenTelemetryHook{tr: tr}
	return otelHook
}

func (h *OpenTelemetryHook) BeforeProcessMessage(evt *taskq.ProcessMessageEvent) error {

	txn := h.tr.StartTransaction("taskq_Event")
	defer txn.End()
	ctx := h.tr.NewContext(context.Background(), txn)

	evt.Message.Ctx = ctx
	return nil
}

func (h *OpenTelemetryHook) AfterProcessMessage(evt *taskq.ProcessMessageEvent) error {
	return nil
}
