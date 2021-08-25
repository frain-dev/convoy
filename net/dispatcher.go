package net

import (
	"bytes"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strings"
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

	response, err := d.client.Do(req)
	ip, u := GetIPAndUserAgent(req)
	r.Response = response
	r.IP = ip
	r.UserAgent = u
	if err != nil {
		log.Errorf("error sending request to API endpoint - %+v\n", err)
		return r, err
	}

	// Close the connection to reuse it
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	r.Body = body
	if err != nil {
		log.Errorf("Couldn't parse Response Body. %+v\n", err)
		return r, err
	}

	return r, nil
}

type Response struct {
	Response  *http.Response
	Body      []byte
	IP        string
	UserAgent string
}

func GetIPAndUserAgent(r *http.Request) (ip string, userAgent string) {
	ip = r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = strings.Split(r.RemoteAddr, ":")[0]
	}

	userAgent = r.UserAgent()
	return ip, userAgent

}
