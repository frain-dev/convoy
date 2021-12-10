package tracer

import (
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

type Tracer interface {
	StartTransaction(name string) *newrelic.Transaction
	SetWebRequestHTTP(r *http.Request, txn *newrelic.Transaction)
	SetWebResponse(w http.ResponseWriter, txn *newrelic.Transaction) http.ResponseWriter
	RequestWithTransactionContext(r *http.Request, txn *newrelic.Transaction) *http.Request
}
