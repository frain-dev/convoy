package net

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type Dispatcher struct {
	client *http.Client
}

func NewDispatcher(httpProxy string, licenser license.Licenser, enforceSecure bool) (*Dispatcher, error) {
	d := &Dispatcher{client: &http.Client{}}

	tr := &http.Transport{
		MaxIdleConns:          100,
		IdleConnTimeout:       10 * time.Second,
		MaxIdleConnsPerHost:   10,
		TLSHandshakeTimeout:   3 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if licenser.UseForwardProxy() {
		proxyUrl, isValid, err := d.setProxy(httpProxy)
		if err != nil {
			return nil, err
		}

		if isValid {
			tr.Proxy = http.ProxyURL(proxyUrl)
		}
	}

	// if enforceSecure is false, allow self-signed certificates, susceptible to MITM attacks.
	// if !enforceSecure {
	//	tr.TLSClientConfig = &tls.Config{
	//		InsecureSkipVerify: true,
	//	}
	// } else {
	//	tr.TLSClientConfig = &tls.Config{
	//		MinVersion: tls.VersionTLS12,
	//	}
	// }

	d.client.Transport = otelhttp.NewTransport(http.DefaultTransport)

	return d, nil
}

func (d *Dispatcher) setProxy(proxyURL string) (*url.URL, bool, error) {
	if !util.IsStringEmpty(proxyURL) {
		pUrl, err := url.Parse(proxyURL)
		if err != nil {
			return nil, false, err
		}

		// we should only use the proxy if the url is valid
		if !util.IsStringEmpty(pUrl.Host) && !util.IsStringEmpty(pUrl.Scheme) {
			return pUrl, true, nil
		}

		return pUrl, false, nil
	}
	return nil, false, nil
}

func (d *Dispatcher) SendRequest(ctx context.Context, endpoint, method string, jsonData json.RawMessage, signatureHeader string, hmac string, maxResponseSize int64, headers httpheader.HTTPHeader, idempotencyKey string, timeout time.Duration) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r := &Response{}
	if util.IsStringEmpty(signatureHeader) || util.IsStringEmpty(hmac) {
		err := errors.New("signature header and hmac are required")
		log.WithError(err).Error("Dispatcher invalid arguments")
		r.Error = err.Error()
		return r, err
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		log.WithError(err).Error("error occurred while creating request")
		return r, err
	}

	req.Header.Set(signatureHeader, hmac)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", defaultUserAgent())
	if len(idempotencyKey) > 0 {
		req.Header.Set("X-Convoy-Idempotency-Key", idempotencyKey)
	}

	header := httpheader.HTTPHeader(req.Header)
	header.MergeHeaders(headers)

	req.Header = http.Header(header)

	r.RequestHeader = req.Header
	r.URL = req.URL
	r.Method = req.Method

	err = d.do(req, r, maxResponseSize)

	return r, err
}

type Response struct {
	Status         string
	StatusCode     int
	Method         string
	URL            *url.URL
	RequestHeader  http.Header
	ResponseHeader http.Header
	Body           []byte
	IP             string
	Error          string
}

func updateDispatchHeaders(r *Response, res *http.Response) {
	r.Status = res.Status
	r.StatusCode = res.StatusCode
	r.ResponseHeader = res.Header
}

// TODO(subomi): Refactor this to support Enterprise Editions
func defaultUserAgent() string {
	return "Convoy/" + convoy.GetVersion()
}

func (d *Dispatcher) do(req *http.Request, res *Response, maxResponseSize int64) error {
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			res.IP = connInfo.Conn.RemoteAddr().String()
			log.Debugf("IP address resolved to: %s", connInfo.Conn.RemoteAddr())
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	response, err := d.client.Do(req)
	if err != nil {
		log.WithError(err).Error("error sending request to API endpoint")
		res.Error = err.Error()
		return err
	}
	defer response.Body.Close()

	updateDispatchHeaders(res, response)

	// io.LimitReader will attempt to read from response.Body until maxResponseSize is reached.
	// if response.Body's length is less than maxResponseSize. body.Read will return io.EOF,
	// if it is greater than maxResponseSize. body.Read will return io.EOF,
	// if it is equal to maxResponseSize. body.Read will return io.EOF,
	// in all cases, io.ReadAll ignores io.EOF.
	body := io.LimitReader(response.Body, maxResponseSize)
	buf, err := io.ReadAll(body)
	res.Body = buf

	if err != nil {
		log.WithError(err).Error("couldn't parse response body")
		return err
	}

	return nil
}
