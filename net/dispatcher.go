package net

import (
	"bytes"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"time"
)

type Dispatcher struct {
	client *http.Client
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *Dispatcher) SendRequest(endpoint, method string, jsonData json.RawMessage) (*Response, error) {
	r := &Response{}
	req, err := http.NewRequest(method, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Errorf("error occurred while creating request - %+v\n", err)
		return r, err
	}

	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			r.IP = connInfo.Conn.RemoteAddr().String()
			log.Debugf("IP address resolved to: %s\n", connInfo.Conn.RemoteAddr())
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	response, err := d.client.Do(req)
	if err != nil {
		log.Debugf("error sending request to API endpoint - %+v\n", err)
		r.Error = err.Error()
		return r, err
	}
	updateDispatcherResponse(r, response)

	body, err := ioutil.ReadAll(response.Body)
	r.Body = body
	if err != nil {
		log.Errorf("Couldn't parse Response Body. %+v\n", err)
		return r, err
	}
	err = response.Body.Close()
	if err != nil {
		log.Errorf("error while closing connection - %+v\n", err)
		return r, err
	}

	return r, nil
}

type Response struct {
	Status      string
	StatusCode  int
	ContentType string
	Header      http.Header
	Body        []byte
	IP          string
	Error       string
}

func updateDispatcherResponse(r *Response, res *http.Response) {
	r.Status = res.Status
	r.StatusCode = res.StatusCode
	r.Header = res.Header
	r.ContentType = res.Header.Get("content-type")
}
