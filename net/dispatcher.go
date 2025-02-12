package net

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/frain-dev/convoy/internal/pkg/fflag"

	"io"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"time"

	"github.com/stealthrocket/netjail"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrAllowListIsRequired = errors.New("allowlist is required")
	ErrBlockListIsRequired = errors.New("blocklist is required")
	ErrLoggerIsRequired    = errors.New("logger is required")
	ErrInvalidIPPrefix     = errors.New("invalid IP prefix")
	ErrTracerIsRequired    = errors.New("tracer cannot be nil")
)

type DispatcherOption func(d *Dispatcher) error

// CustomTransport wraps both netjail.Transport and otelhttp.Transport
type CustomTransport struct {
	otelTransport    *otelhttp.Transport
	netJailTransport *netjail.Transport
	vanillaTransport *http.Transport
}

// RoundTrip executes a single HTTP transaction
func (c *CustomTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return c.otelTransport.RoundTrip(req)
}

// NewNetJailTransport creates a new CustomTransport with a netJailTransport
func NewNetJailTransport(netJailTransport *netjail.Transport) *CustomTransport {
	otelTransport := otelhttp.NewTransport(
		netJailTransport,
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return fmt.Sprintf("%s webhook.dispatch", r.Method)
		}),
		otelhttp.WithFilter(func(r *http.Request) bool {
			return true
		}),
	)
	return &CustomTransport{
		otelTransport:    otelTransport,
		netJailTransport: netJailTransport,
	}
}

// NewVanillaTransport creates a new CustomTransport with a default transport
func NewVanillaTransport(transport *http.Transport) *CustomTransport {
	otelTransport := otelhttp.NewTransport(
		transport,
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return fmt.Sprintf("%s webhook.dispatch", r.Method)
		}),
		otelhttp.WithFilter(func(r *http.Request) bool {
			return true
		}),
	)
	return &CustomTransport{
		otelTransport:    otelTransport,
		vanillaTransport: transport,
	}
}

type DetailedTraceConfig struct {
	Enabled bool
}

type Dispatcher struct {
	// gating mechanisms
	ff *fflag.FFlag
	l  license.Licenser

	logger        *log.Logger
	transport     *http.Transport
	client        *http.Client
	rules         *netjail.Rules
	tracer        tracer.Backend
	detailedTrace DetailedTraceConfig
}

func NewDispatcher(l license.Licenser, ff *fflag.FFlag, options ...DispatcherOption) (*Dispatcher, error) {
	d := &Dispatcher{
		ff:     ff,
		l:      l,
		logger: log.NewLogger(os.Stdout),
		tracer: tracer.NoOpBackend{},
		client: &http.Client{},
		rules:  &netjail.Rules{},
		transport: &http.Transport{
			MaxIdleConns:          1000,
			IdleConnTimeout:       30 * time.Second,
			MaxIdleConnsPerHost:   100,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DisableCompression:    false,
		},
	}

	for _, option := range options {
		if err := option(d); err != nil {
			return nil, err
		}
	}

	if d.logger == nil {
		return nil, ErrLoggerIsRequired
	}

	netJailTransport := &netjail.Transport{
		New: func() *http.Transport {
			return d.transport.Clone()
		},
	}

	if ff.CanAccessFeature(fflag.IpRules) && l.IpRules() {
		d.client.Transport = NewNetJailTransport(netJailTransport)
	} else {
		d.client.Transport = NewVanillaTransport(d.transport)
	}

	return d, nil
}

// ProxyOption defines an HTTP proxy which the client will use. It fails-open the string isn't a valid HTTP URL
func ProxyOption(httpProxy string) DispatcherOption {
	return func(d *Dispatcher) error {
		if httpProxy == "" {
			return nil
		}

		if d.l.UseForwardProxy() {
			proxyUrl, isValid, err := d.validateProxy(httpProxy)
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

// AllowListOption sets a list of IP prefixes which will outgoing traffic will be granted access
func AllowListOption(allowList []string) DispatcherOption {
	return func(d *Dispatcher) error {
		if !d.l.IpRules() || !d.ff.CanAccessFeature(fflag.IpRules) {
			return nil
		}

		if len(allowList) == 0 {
			return ErrAllowListIsRequired
		}

		netAllow := make([]netip.Prefix, len(allowList))
		for i, prefix := range allowList {
			parsed, err := netip.ParsePrefix(prefix)
			if err != nil {
				return fmt.Errorf("%w: %v in allowlist", ErrInvalidIPPrefix, err)
			}
			netAllow[i] = parsed
			d.rules.Allow = netAllow
		}

		return nil
	}
}

// BlockListOption sets a list of IP prefixes which will outgoing traffic will be denied access
func BlockListOption(blockList []string) DispatcherOption {
	return func(d *Dispatcher) error {
		if !d.l.IpRules() || !d.ff.CanAccessFeature(fflag.IpRules) {
			return nil
		}

		if len(blockList) == 0 {
			return ErrBlockListIsRequired
		}

		netBlock := make([]netip.Prefix, len(blockList))
		for i, prefix := range blockList {
			parsed, err := netip.ParsePrefix(prefix)
			if err != nil {
				return fmt.Errorf("%w: %v in blocklist", ErrInvalidIPPrefix, err)
			}
			netBlock[i] = parsed
		}

		d.rules.Block = netBlock
		return nil
	}
}

// InsecureSkipVerifyOption allow self-signed certificates
// to be used if set to true but is susceptible to Man In The Middle attacks.
func InsecureSkipVerifyOption(insecureSkipVerify bool) DispatcherOption {
	return func(d *Dispatcher) error {
		if insecureSkipVerify {
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

// TracerOption sets a custom tracer backend for the Dispatcher
func TracerOption(tracer tracer.Backend) DispatcherOption {
	return func(d *Dispatcher) error {
		if tracer == nil {
			return ErrTracerIsRequired
		}

		d.tracer = tracer
		return nil
	}
}

func DetailedTraceOption(enabled bool) DispatcherOption {
	return func(d *Dispatcher) error {
		d.detailedTrace.Enabled = enabled
		return nil
	}
}

func (d *Dispatcher) validateProxy(proxyURL string) (*url.URL, bool, error) {
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
	d.logger.Debugf("rules: %+v", d.rules)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r := &Response{}
	if util.IsStringEmpty(signatureHeader) || util.IsStringEmpty(hmac) {
		err := errors.New("signature header and hmac are required")
		d.logger.WithError(err).Error("Dispatcher invalid arguments")
		r.Error = err.Error()
		return r, err
	}

	if d.ff.CanAccessFeature(fflag.IpRules) && d.l.IpRules() {
		ctx = netjail.ContextWithRules(ctx, d.rules)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		d.logger.WithError(err).Error("error occurred while creating request")
		return r, err
	}

	req.Header.Set(signatureHeader, hmac)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
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

	err = d.do(ctx, req, r, maxResponseSize)
	if err != nil {
		return r, err
	}

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

func (d *Dispatcher) do(ctx context.Context, req *http.Request, res *Response, maxResponseSize int64) error {
	if d.detailedTrace.Enabled {
		trace := &httptrace.ClientTrace{
			DNSStart: func(info httptrace.DNSStartInfo) {
				attrs := map[string]interface{}{
					"dns.host": info.Host,
					"event":    "dns_start",
				}
				d.tracer.Capture(ctx, "dns_lookup_start", attrs, time.Now(), time.Now())
			},
			DNSDone: func(info httptrace.DNSDoneInfo) {
				attrs := map[string]interface{}{
					"dns.addresses": fmt.Sprintf("%v", info.Addrs),
					"dns.error":     fmt.Sprintf("%v", info.Err),
					"event":         "dns_done",
				}
				d.tracer.Capture(ctx, "dns_lookup_done", attrs, time.Now(), time.Now())
			},
			ConnectStart: func(network, addr string) {
				attrs := map[string]interface{}{
					"net.network": network,
					"net.addr":    addr,
					"event":       "connect_start",
				}
				d.tracer.Capture(ctx, "connect_start", attrs, time.Now(), time.Now())
			},
			ConnectDone: func(network, addr string, err error) {
				attrs := map[string]interface{}{
					"net.network": network,
					"net.addr":    addr,
					"error":       fmt.Sprintf("%v", err),
					"event":       "connect_done",
				}
				d.tracer.Capture(ctx, "connect_done", attrs, time.Now(), time.Now())
			},
			TLSHandshakeStart: func() {
				attrs := map[string]interface{}{
					"event": "tls_handshake_start",
				}
				d.tracer.Capture(ctx, "tls_handshake_start", attrs, time.Now(), time.Now())
			},
			TLSHandshakeDone: func(state tls.ConnectionState, err error) {
				attrs := map[string]interface{}{
					"tls.version":      state.Version,
					"tls.cipher_suite": state.CipherSuite,
					"error":            fmt.Sprintf("%v", err),
					"event":            "tls_handshake_done",
				}
				d.tracer.Capture(ctx, "tls_handshake_done", attrs, time.Now(), time.Now())
			},
			GotFirstResponseByte: func() {
				attrs := map[string]interface{}{
					"event": "first_byte_received",
				}
				d.tracer.Capture(ctx, "first_byte", attrs, time.Now(), time.Now())
			},
		}

		ctx = httptrace.WithClientTrace(ctx, trace)
		req = req.WithContext(ctx)
	}

	response, err := d.client.Do(req)
	if err != nil {
		d.logger.WithError(err).Error("error sending request to API endpoint")
		res.Error = err.Error()
		return err
	}
	defer response.Body.Close()

	// io.LimitReader will attempt to read from response.Body until maxResponseSize is reached.
	// if response.Body's length is less than maxResponseSize. body.Read will return io.EOF,
	// if it is greater than maxResponseSize. body.Read will return io.EOF,
	// if it is equal to maxResponseSize. body.Read will return io.EOF,
	// in all cases, io.ReadAll ignores io.EOF.
	body := io.LimitReader(response.Body, maxResponseSize)

	var reader io.Reader
	// Check if response is gzipped
	if response.Header.Get("Content-Encoding") == "gzip" {
		gzReader, readErr := gzip.NewReader(body)
		if readErr != nil {
			return readErr
		}
		defer gzReader.Close()
		reader = gzReader
	} else {
		reader = body
	}

	buf, err := io.ReadAll(reader)
	res.Body = buf

	updateDispatchHeaders(res, response)

	if err != nil {
		d.logger.WithError(err).Error("couldn't parse response body")
		return err
	}

	return nil
}
