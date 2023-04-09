package net

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type Dispatcher struct {
	client *http.Client
}

func NewDispatcher(timeout time.Duration, httpProxy string) (*Dispatcher, error) {
	d := &Dispatcher{client: &http.Client{Timeout: timeout}}

	if len(httpProxy) > 0 {
		proxyUrl, err := url.Parse(httpProxy)
		if err != nil {
			return nil, err
		}

		d.client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
	}

	return d, nil
}

func (d *Dispatcher) SendRequest(endpoint, method string, jsonData json.RawMessage, project *datastore.Project, hmac string, maxResponseSize int64, headers httpheader.HTTPHeader) (*Response, error) {
	r := &Response{}
	signatureHeader := project.Config.Signature.Header.String()
	if util.IsStringEmpty(signatureHeader) || util.IsStringEmpty(hmac) {
		err := errors.New("signature header and hmac are required")
		log.WithError(err).Error("Dispatcher invalid arguments")
		r.Error = err.Error()
		return r, err
	}

	req, err := http.NewRequest(method, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		log.WithError(err).Error("error occurred while creating request")
		return r, err
	}

	req.Header.Set(signatureHeader, hmac)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", defaultUserAgent())

	header := httpheader.HTTPHeader(req.Header)
	header.MergeHeaders(headers)

	req.Header = http.Header(header)

	r.RequestHeader = req.Header
	r.URL = req.URL
	r.Method = req.Method

	err = d.do(req, r, maxResponseSize)

	return r, err
}

func (d *Dispatcher) SendCliRequest(url string, method convoy.HttpMethod, apiKey string, jsonData json.RawMessage) (*Response, error) {
	r := &Response{}

	req, err := http.NewRequest(string(method), url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.WithError(err).Error("error occurred while creating request")
		return r, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", defaultUserAgent())
	req.Header.Add("Authorization", fmt.Sprintf(" Bearer %s", apiKey))

	r.RequestHeader = req.Header
	r.URL = req.URL
	r.Method = req.Method

	err = d.do(req, r, config.MaxRequestSize)

	return r, err
}

func (d *Dispatcher) ForwardCliEvent(url string, method convoy.HttpMethod, jsonData json.RawMessage, headers httpheader.HTTPHeader) (*Response, error) {
	r := &Response{}

	req, err := http.NewRequest(string(method), url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.WithError(err).Error("error occurred while creating request")
		return r, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", defaultUserAgent())

	header := httpheader.HTTPHeader(req.Header)
	header.MergeHeaders(headers)

	req.Header = http.Header(header)

	r.RequestHeader = req.Header
	r.URL = req.URL
	r.Method = req.Method

	err = d.do(req, r, config.MaxRequestSize)

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
			log.Infof("IP address resolved to: %s", connInfo.Conn.RemoteAddr())
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	response, err := d.client.Do(req)
	if err != nil {
		log.WithError(err).Error("error sending request to API endpoint")
		res.Error = err.Error()
		return err
	}
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
	defer response.Body.Close()

	return nil
}
