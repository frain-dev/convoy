package flipt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/oklog/ulid/v2"
)

var (
	ErrFliptServerError  = errors.New("something went wrong with the flipt server")
	ErrFliptFlagNotFound = errors.New("flag not found")
)

type Flipt struct {
	client *http.Client
	host   string
}

func NewFliptClient(host string) *Flipt {
	return &Flipt{
		client: &http.Client{Timeout: 10 * time.Second},
		host:   host,
	}
}

func (f *Flipt) IsEnabled(flagKey string, evaluate map[string]string) (bool, error) {
	flag, err := f.getFlag(flagKey)
	if err != nil {
		return false, err
	}

	// The flag not being enabled means everybody has
	// access to that feature
	if !flag.Enabled {
		return true, nil
	}

	result, err := f.evaluate(flagKey, evaluate)
	if err != nil {
		return false, err
	}

	return result.Match, nil
}

func (f *Flipt) getFlag(flagKey string) (*FlagResponse, error) {
	response, err := f.SendRequest(http.MethodGet, fmt.Sprintf("flags/%s", flagKey), nil)
	if err != nil {
		return nil, err
	}

	statusCode := response.StatusCode

	if statusCode == 200 {
		var flag *FlagResponse
		err := json.Unmarshal(response.Body, &flag)
		if err != nil {
			return nil, err
		}

		return flag, nil
	} else if statusCode == 400 {
		return nil, ErrFliptFlagNotFound
	}

	return nil, ErrFliptServerError
}

func (f *Flipt) evaluate(flagKey string, evaluate map[string]string) (*EvaluateResponse, error) {
	body := struct {
		RequestId string            `json:"requestId"`
		FlagKey   string            `json:"flagKey"`
		EntityId  string            `json:"entityId"`
		Context   map[string]string `json:"context"`
	}{
		RequestId: ulid.Make().String(),
		FlagKey:   flagKey,
		EntityId:  ulid.Make().String(),
		Context:   evaluate,
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	response, err := f.SendRequest(http.MethodPost, "evaluate", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	statusCode := response.StatusCode

	if statusCode == 200 {
		var evaluateResponse *EvaluateResponse
		err := json.Unmarshal(response.Body, &evaluateResponse)
		if err != nil {
			return nil, err
		}

		return evaluateResponse, nil
	} else if statusCode == 400 {
		return nil, ErrFliptFlagNotFound
	}

	return nil, ErrFliptServerError
}

func (f *Flipt) SendRequest(method, path string, body io.Reader) (*Response, error) {
	url := fmt.Sprintf("%s/api/v1/%s", f.host, path)
	req, err := http.NewRequest(method, url, body)

	if err != nil {
		return nil, err
	}

	response, err := f.client.Do(req)

	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	rBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:     response.Status,
		StatusCode: response.StatusCode,
		Body:       rBody,
	}, nil
}

func BatchEvaluate(w http.ResponseWriter, r *http.Request) {
	config, err := config.Get()
	if err != nil {
		log.Fatal(err)
	}

	url, err := url.Parse(config.FeatureFlag.Flipt.Host)
	if err != nil {
		log.Fatal(err)
	}

	// create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(url)

	// update the headers to allow for SSL redirection
	r.URL.Host = url.Host
	r.URL.Scheme = url.Scheme
	r.URL.Path = "/api/v1/batch-evaluate"
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = url.Host

	proxy.ServeHTTP(w, r)
}

type Response struct {
	Status     string
	StatusCode int
	Method     string
	Body       []byte
}

type FlagResponse struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

type EvaluateResponse struct {
	FlagKey string `json:"flagKey"`
	Match   bool   `json:"match"`
}
