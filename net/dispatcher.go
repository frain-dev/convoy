package net

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"
)

type UserAgent string

const (
	DefaultUserAgent UserAgent = "Convoy/v0.2"
)

type Dispatcher struct {
	client *http.Client
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *Dispatcher) SendRequest(endpoint, method string, jsonData json.RawMessage, signatureHeader string, hmac string) (*Response, error) {
	r := &Response{}

	if util.IsStringEmpty(signatureHeader) || util.IsStringEmpty(hmac) {
		err := errors.New("signature header and hmac are required")
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
	req.Header.Add("User-Agent", string(DefaultUserAgent))

	r.RequestHeader = req.Header
	r.URL = req.URL
	r.Method = req.Method

	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			r.IP = connInfo.Conn.RemoteAddr().String()
			log.Infof("IP address resolved to: %s", connInfo.Conn.RemoteAddr())
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	response, err := d.client.Do(req)
	if err != nil {
		log.WithError(err).Error("error sending request to API endpoint")
		r.Error = err.Error()
		return r, err
	}
	updateDispatchHeaders(r, response)

	body, err := ioutil.ReadAll(response.Body)
	r.Body = body
	if err != nil {
		log.WithError(err).Error("couldn't parse response body")
		return r, err
	}
	err = response.Body.Close()
	if err != nil {
		log.WithError(err).Error("error while closing connection")
		return r, err
	}

	return r, nil
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
