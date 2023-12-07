package apm

import (
	"context"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"net/http"
)

type APM interface {
	StartWebTransaction(name string, r *http.Request, w http.ResponseWriter) (Transaction, *http.Request, http.ResponseWriter)
	StartTransaction(ctx context.Context, name string) (Transaction, context.Context)
	Shutdown()
}

type Transaction interface {
	AddTag(key string, value interface{})
	End()
	RecordError(err error)
}

// TransactionOption defines an option for configuring transactions.
type TransactionOption func(*TransactionOptions)

// TransactionOptions holds configuration for transactions.
type TransactionOptions struct {
	// Additional fields as needed.
}

var (
	std = apmImpl{}
)

type apmImpl struct {
	//nr *newRelicAPM
	dd *dataDogAPM
}

func NoticeError(ctx context.Context, err error) {
	//txn := newrelic.FromContext(ctx)
	//txn.NoticeError(err)

	span, _ := tracer.StartSpanFromContext(ctx, "")
	span.Finish(tracer.WithError(err))
}

func SetApplication(app APM) {
	//if a, ok := app.(*newRelicAPM); ok {
	//	std.nr = a
	//}

	if d, ok := app.(*dataDogAPM); ok {
		std.dd = d
	}
}

func StartTransaction(ctx context.Context, name string) (Transaction, context.Context) {
	return std.dd.StartTransaction(ctx, name)
}

func StartWebTransaction(name string, r *http.Request, w http.ResponseWriter) (Transaction, *http.Request, http.ResponseWriter) {
	return std.dd.StartWebTransaction(name, r, w)
}
