package net

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/stealthrocket/netjail"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrAllowListIsRequired = errors.New("allowlist is required")
	ErrBlockListIsRequired = errors.New("blocklist is required")
	ErrLoggerIsRequired    = errors.New("logger is required")
)

type DispatcherOption func(d *Dispatcher) error

type Dispatcher struct {
	logger    *log.Logger
	transport *http.Transport
	client    *http.Client
	rules     *netjail.Rules
}

func ProxyOption(l license.Licenser, httpProxy string) DispatcherOption {
	return func(d *Dispatcher) error {
		if httpProxy == "" {
			return nil
		}

		if l.UseForwardProxy() {
			proxyUrl, isValid, err := d.setProxy(httpProxy)
			if err != nil {
				return err
			}

			if isValid {
				d.transport.Proxy = http.ProxyURL(proxyUrl)
			}
		}

		return nil
	}
}

func AllowListOption(allowList []string) DispatcherOption {
	return func(d *Dispatcher) error {
		if allowList == nil || len(allowList) == 0 {
			return ErrAllowListIsRequired
		}

		netAllow := make([]netip.Prefix, len(allowList))
		for i := range allowList {
			prefix := netip.MustParsePrefix(allowList[i])
			netAllow[i] = prefix
		}

		d.rules.Allow = netAllow
		return nil
	}
}

func BlockListOption(blockList []string) DispatcherOption {
	return func(d *Dispatcher) error {
		if blockList == nil || len(blockList) == 0 {
			return ErrBlockListIsRequired
		}

		netBlock := make([]netip.Prefix, len(blockList))
		for i := range blockList {
			prefix := netip.MustParsePrefix(blockList[i])
			netBlock[i] = prefix
		}

		d.rules.Block = netBlock
		return nil
	}
}

// EnforceSecureOption allow self-signed certificates if set to false
// which is susceptible to MITM attacks.
func EnforceSecureOption(enforceSecure bool) DispatcherOption {
	return func(d *Dispatcher) error {
		if !enforceSecure {
			d.transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		} else {
			d.transport.TLSClientConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		}

		return nil
	}
}

func LoggerOption(logger *log.Logger) DispatcherOption {
	return func(d *Dispatcher) error {
		if logger == nil {
			return ErrLoggerIsRequired
		}

		d.logger = logger
		return nil
	}
}

func NewDispatcher(options ...DispatcherOption) (*Dispatcher, error) {
	d := &Dispatcher{
		client: &http.Client{},
		rules:  &netjail.Rules{},
		transport: &http.Transport{
			MaxIdleConns:          100,
			IdleConnTimeout:       10 * time.Second,
			MaxIdleConnsPerHost:   10,
			TLSHandshakeTimeout:   3 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	for _, option := range options {
		if err := option(d); err != nil {
			return nil, err
		}
	}

	netJailTransport := &netjail.Transport{
		New: func() *http.Transport {
			return d.transport.Clone()
		},
	}
	d.client.Transport = netJailTransport

	if d.logger == nil {
		return nil, ErrLoggerIsRequired
	}

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
		d.logger.WithError(err).Error("Dispatcher invalid arguments")
		r.Error = err.Error()
		return r, err
	}

	req, err := http.NewRequestWithContext(netjail.ContextWithRules(ctx, d.rules), method, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		d.logger.WithError(err).Error("error occurred while creating request")
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

func defaultUserAgent() string {
	return "Convoy/" + convoy.GetVersion()
}

func (d *Dispatcher) do(req *http.Request, res *Response, maxResponseSize int64) error {
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			res.IP = connInfo.Conn.RemoteAddr().String()
			d.logger.Debugf("IP address resolved to: %s", connInfo.Conn.RemoteAddr())
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	response, err := d.client.Do(req)
	if err != nil {
		d.logger.WithError(err).Error("error sending request to API endpoint")
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
		d.logger.WithError(err).Error("couldn't parse response body")
		return err
	}

	return nil
}
