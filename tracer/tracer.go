package tracer

import (
	"context"
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/config"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/sirupsen/logrus"
)

type Tracer interface {
	StartTransaction(name string) *newrelic.Transaction
	SetWebRequestHTTP(r *http.Request, txn *newrelic.Transaction)
	SetWebResponse(w http.ResponseWriter, txn *newrelic.Transaction) http.ResponseWriter
	RequestWithTransactionContext(r *http.Request, txn *newrelic.Transaction) *http.Request
	NewContext(ctx context.Context, txn *newrelic.Transaction) context.Context
}

func NewTracer(cfg config.Configuration, logger *logrus.Logger) (Tracer, error) {
	if cfg.Tracer.Type != config.NewRelicTracerProvider {
		return nil, errors.New("Tracer is not supported")
	}

	switch cfg.Tracer.Type {
	case config.NewRelicTracerProvider:
		tr, err := NewNRClient(cfg.Tracer.NewRelic, logger)

		if err != nil {
			return nil, err
		}

		return tr, nil
	}

	return nil, nil
}
