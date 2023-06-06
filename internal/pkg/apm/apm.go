package apm

import (
	"context"
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

var (
	std = New()
)

func SetApplication(app *newrelic.Application) {
	std.SetApplication(app)
}

func NoticeError(ctx context.Context, err error) {
	std.NoticeError(ctx, err)
}

func StartTransaction(ctx context.Context, name string) (*Transaction, context.Context) {
	return std.StartTransaction(ctx, name)
}

func StartWebTransaction(name string, r *http.Request, w http.ResponseWriter) (*Transaction, *http.Request, http.ResponseWriter) {
	return std.StartWebTransaction(name, r, w)
}

type APM struct {
	application *newrelic.Application
}

func New() *APM {
	return &APM{}
}

func (a *APM) SetApplication(app *newrelic.Application) {
	a.application = app
}

func (a *APM) NoticeError(ctx context.Context, err error) {
	txn := newrelic.FromContext(ctx)
	txn.NoticeError(err)
}

func (a *APM) StartTransaction(ctx context.Context, name string) (*Transaction, context.Context) {
	inner := a.createTransaction(name)
	c := newrelic.NewContext(ctx, inner)

	return NewTransaction(inner), c
}

func (a *APM) StartWebTransaction(name string, r *http.Request, w http.ResponseWriter) (*Transaction, *http.Request, http.ResponseWriter) {
	inner := a.createTransaction(name)

	// Set the transaction as a web request, gather attributes based on the
	// request, and read incoming distributed trace headers.
	inner.SetWebRequestHTTP(r)

	// Prepare to capture attributes, errors, and headers from the
	// response.
	w = inner.SetWebResponse(w)

	// Add the Transaction to the http.Request's Context.
	r = newrelic.RequestWithTransactionContext(r, inner)

	// Encapsulate Transaction
	txn := NewTransaction(inner)

	return txn, r, w
}

func (a *APM) createTransaction(name string) *newrelic.Transaction {
	return a.application.StartTransaction(name)
}

type Transaction struct {
	txn *newrelic.Transaction
}

func NewTransaction(inner *newrelic.Transaction) *Transaction {
	return &Transaction{inner}
}

func (t *Transaction) End() {
	t.txn.End()
}
