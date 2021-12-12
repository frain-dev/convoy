package tracer

import (
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/util"
	"github.com/newrelic/go-agent/v3/integrations/nrlogrus"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/sirupsen/logrus"
)

type NewRelicClient struct {
	Application *newrelic.Application
}

func NewNRClient(cfg config.TracerConfiguration, logger *logrus.Logger) (*NewRelicClient, error) {
	if util.IsStringEmpty(cfg.NewRelic.LicenseKey) {
		return nil, errors.New("please provide the New Relic License Key")
	}

	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(cfg.NewRelic.AppName),
		newrelic.ConfigLicense(cfg.NewRelic.LicenseKey),
		newrelic.ConfigDistributedTracerEnabled(cfg.NewRelic.DistributedTracerEnabled),
		newrelic.ConfigEnabled(cfg.NewRelic.ConfigEnabled),
		nrlogrus.ConfigLogger(logger),
	)

	if err != nil {
		return nil, err
	}

	nr := &NewRelicClient{Application: app}

	return nr, nil
}

func (nr *NewRelicClient) StartTransaction(name string) *newrelic.Transaction {
	return nr.Application.StartTransaction(name)
}

func (nr *NewRelicClient) SetWebRequestHTTP(r *http.Request, txn *newrelic.Transaction) {
	txn.SetWebRequestHTTP(r)
}

func (nr *NewRelicClient) SetWebResponse(w http.ResponseWriter, txn *newrelic.Transaction) http.ResponseWriter {
	return txn.SetWebResponse(w)
}

func (nr *NewRelicClient) RequestWithTransactionContext(r *http.Request, txn *newrelic.Transaction) *http.Request {
	return newrelic.RequestWithTransactionContext(r, txn)
}
