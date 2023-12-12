package apm

import (
	"context"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"net/http"
)

// dataDogAPM is an implementation of the APM interface for DataDog.
type dataDogAPM struct{}

// NewDataDogAPM initializes a new DataDog APM instance.
func NewDataDogAPM(serviceName string, opts ...tracer.StartOption) (APM, error) {
	// Additional configuration options can be added here
	tracer.Start(tracer.WithServiceName(serviceName), tracer.WithEnv("dev"))
	return &dataDogAPM{}, nil
}

func (d *dataDogAPM) StartWebTransaction(name string, r *http.Request, w http.ResponseWriter) (Transaction, *http.Request, http.ResponseWriter) {
	span, _ := tracer.StartSpanFromContext(r.Context(), name)
	return &dataDogTransaction{span: span}, r, w
}

func (d *dataDogAPM) StartTransaction(ctx context.Context, name string) (Transaction, context.Context) {
	span, ctxt := tracer.StartSpanFromContext(ctx, name)
	return &dataDogTransaction{span: span}, ctxt
}

func (d *dataDogAPM) Shutdown() {
	tracer.Stop() // Stop the tracer when you shut down your application
}

// dataDogTransaction is an implementation of the Transaction interface for DataDog.
type dataDogTransaction struct {
	span ddtrace.Span
}

func (dt *dataDogTransaction) AddTag(key string, value interface{}) {
	dt.span.SetTag(key, value)
}

func (dt *dataDogTransaction) End() {
	dt.span.Finish()
}

func (dt *dataDogTransaction) RecordError(err error) {
	dt.span.SetTag("error", err)
}
