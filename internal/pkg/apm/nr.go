package apm

import (
	"context"
	"github.com/newrelic/go-agent/v3/newrelic"
	"net/http"
	"time"
)

// newRelicAPM is an implementation of the APM interface for New Relic.
type newRelicAPM struct {
	app *newrelic.Application
}

// NewNewRelicAPM creates a new instance of New Relic APM.
func NewNewRelicAPM(appName, licenseKey string, configEnabled, distributedTracerEnabled bool) (APM, error) {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(appName),
		newrelic.ConfigLicense(licenseKey),
		newrelic.ConfigEnabled(configEnabled),
		newrelic.ConfigAppLogForwardingEnabled(true),
		newrelic.ConfigDistributedTracerEnabled(distributedTracerEnabled),
	)

	if err != nil {
		return nil, err
	}
	return &newRelicAPM{app: app}, nil
}

func (nr *newRelicAPM) StartWebTransaction(name string, r *http.Request, w http.ResponseWriter) (Transaction, *http.Request, http.ResponseWriter) {
	inner := nr.app.StartTransaction(name)

	// Set the transaction as a web request, gather attributes based on the
	// request, and read incoming distributed trace headers.
	inner.SetWebRequestHTTP(r)

	// Prepare to capture attributes, errors, and headers from the
	// response.
	w = inner.SetWebResponse(w)

	// Add the NewRelicTransaction to the http.Request's Context.
	r = newrelic.RequestWithTransactionContext(r, inner)

	// Encapsulate NewRelicTransaction
	txn := NewTransaction(inner)

	return txn, r, w
}

func (nr *newRelicAPM) StartTransaction(ctx context.Context, name string) (Transaction, context.Context) {
	inner := nr.app.StartTransaction(name)
	ctxt := newrelic.NewContext(ctx, inner)
	return NewTransaction(inner), ctxt
}

func (nr *newRelicAPM) Shutdown() {
	nr.app.Shutdown(time.Second * 10)
}

// NewRelicTransaction is an implementation of the Transaction interface for New Relic.
type NewRelicTransaction struct {
	txn *newrelic.Transaction
}

func NewTransaction(txn *newrelic.Transaction) Transaction {
	return &NewRelicTransaction{txn}
}

func (nrt *NewRelicTransaction) AddTag(key string, value interface{}) {
	nrt.txn.AddAttribute(key, value)
}

func (nrt *NewRelicTransaction) End() {
	nrt.txn.End()
}

func (nrt *NewRelicTransaction) RecordError(err error) {
	nrt.txn.NoticeError(err)
}
