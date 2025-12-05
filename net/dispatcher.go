package net

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"os"
	"time"

	"github.com/stealthrocket/netjail"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/constants"
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
	ErrNon2xxResponse      = errors.New("endpoint returned a non-2xx response")
)

// ContentTypeConverter defines the interface for converting JSON data to different content types
type ContentTypeConverter interface {
	Convert(jsonData json.RawMessage) ([]byte, error)
	ContentType() string
}

// JSONConverter handles application/json content type
type JSONConverter struct{}

func (j JSONConverter) Convert(jsonData json.RawMessage) ([]byte, error) {
	return jsonData, nil
}

func (j JSONConverter) ContentType() string {
	return constants.ContentTypeJSON
}

// FormURLEncodedConverter handles application/x-www-form-urlencoded content type
type FormURLEncodedConverter struct{}

func (f FormURLEncodedConverter) Convert(jsonData json.RawMessage) ([]byte, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}

	values := url.Values{}
	for key, value := range data {
		switch v := value.(type) {
		case string:
			values.Set(key, v)
		case float64:
			values.Set(key, fmt.Sprintf("%.0f", v))
		case bool:
			values.Set(key, fmt.Sprintf("%t", v))
		case nil:
			values.Set(key, "")
		default:
			// For complex types, convert to JSON string
			jsonValue, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal value for key %s: %w", key, err)
			}
			values.Set(key, string(jsonValue))
		}
	}

	return []byte(values.Encode()), nil
}

func (f FormURLEncodedConverter) ContentType() string {
	return constants.ContentTypeFormURLEncoded
}

// getConverter returns the appropriate converter for the given content type
func getConverter(contentType string) ContentTypeConverter {
	switch contentType {
	case constants.ContentTypeFormURLEncoded:
		return FormURLEncodedConverter{}
	default:
		return JSONConverter{}
	}
}

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

	logger        log.StdLogger
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

// TLSConfigOption configures TLS settings for the dispatcher.
// If `insecureSkipVerify` is true, it allows self-signed certificates but is vulnerable to MITM attacks.
// If a custom CA is provided, it is used for TLS verification.
// Otherwise, it enforces a secure minimum TLS version.
func TLSConfigOption(insecureSkipVerify bool, licenser license.Licenser, caCertTLSConfig *tls.Config) DispatcherOption {
	return func(d *Dispatcher) error {
		switch {
		case insecureSkipVerify:
			d.transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		case licenser.CustomCertificateAuthority() && caCertTLSConfig != nil:
			d.transport.TLSClientConfig = caCertTLSConfig
		default:
			d.transport.TLSClientConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		}

		return nil
	}
}

func LoggerOption(logger log.StdLogger) DispatcherOption {
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

// createClientWithMTLS creates an HTTP client configured with the provided mTLS certificate.
// If mtlsCert is nil, returns the default dispatcher client.
// The returned client respects IP allow/block rules when the feature is enabled.
func (d *Dispatcher) createClientWithMTLS(mtlsCert *tls.Certificate) *http.Client {
	if mtlsCert == nil {
		return d.client
	}

	customTransport := d.transport.Clone()

	// Clone the TLS config to avoid modifying the shared config
	var tlsConfig *tls.Config
	if customTransport.TLSClientConfig != nil {
		tlsConfig = customTransport.TLSClientConfig.Clone()
	} else {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	// Add the client certificate
	tlsConfig.Certificates = []tls.Certificate{*mtlsCert}
	customTransport.TLSClientConfig = tlsConfig

	// Respect IP allow/block rules by using netjail transport when enabled
	if d.ff.CanAccessFeature(fflag.IpRules) && d.l.IpRules() {
		netJailTransport := &netjail.Transport{
			New: func() *http.Transport { return customTransport.Clone() },
		}
		return &http.Client{Transport: NewNetJailTransport(netJailTransport)}
	}

	return &http.Client{Transport: NewVanillaTransport(customTransport)}
}

// SendWebhookWithMTLS sends a webhook request with optional mTLS client certificate configuration
func (d *Dispatcher) SendWebhookWithMTLS(ctx context.Context, endpoint string, jsonData json.RawMessage,
	signatureHeader, hmac string, maxResponseSize int64, headers httpheader.HTTPHeader,
	idempotencyKey string, timeout time.Duration, contentType string, mtlsCert *tls.Certificate) (*Response, error) {
	client := d.createClientWithMTLS(mtlsCert)
	return d.sendWebhookInternal(ctx, endpoint, jsonData, signatureHeader, hmac, maxResponseSize, headers, idempotencyKey, timeout, contentType, client)
}

func (d *Dispatcher) SendWebhook(ctx context.Context, endpoint string, jsonData json.RawMessage,
	signatureHeader, hmac string, maxResponseSize int64, headers httpheader.HTTPHeader,
	idempotencyKey string, timeout time.Duration, contentType string) (*Response, error) {
	return d.sendWebhookInternal(ctx, endpoint, jsonData, signatureHeader, hmac, maxResponseSize, headers, idempotencyKey, timeout, contentType, d.client)
}

func (d *Dispatcher) sendWebhookInternal(ctx context.Context, endpoint string, jsonData json.RawMessage,
	signatureHeader, hmac string, maxResponseSize int64, headers httpheader.HTTPHeader,
	idempotencyKey string, timeout time.Duration, contentType string, client *http.Client) (*Response, error) {
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

	// Convert JSON data to the appropriate content type using converter interface
	converter := getConverter(contentType)
	requestBody, err := converter.Convert(jsonData)
	if err != nil {
		d.logger.WithError(err).Error("error converting JSON data")
		r.Error = err.Error()
		return r, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		d.logger.WithError(err).Error("error occurred while creating request")
		return r, err
	}

	req.Header.Set(signatureHeader, hmac)
	req.Header.Add("Content-Type", converter.ContentType())
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

	err = d.do(ctx, req, r, maxResponseSize, client)
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

func (d *Dispatcher) do(ctx context.Context, req *http.Request, res *Response, maxResponseSize int64, client *http.Client) error {
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

	response, err := client.Do(req)
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

// OAuth2TokenGetter is a function type for getting OAuth2 access tokens
// It returns the formatted Authorization header value (e.g., "Bearer token" or "CustomType token")
type OAuth2TokenGetter func(ctx context.Context) (string, error)

// PingOptions contains options for the Ping operation
type PingOptions struct {
	Endpoint          string
	Timeout           time.Duration
	ContentType       string
	MtlsCert          *tls.Certificate
	OAuth2TokenGetter OAuth2TokenGetter
	// Method is used internally by tryPingMethod. It's set automatically by Ping.
	Method string
}

// Ping sends requests to the specified endpoint using configurable methods and verifies it returns a 2xx response.
// It returns an error if the endpoint is unreachable or returns a non-2xx status code.
// If opts.OAuth2TokenGetter is provided, it will be used to fetch an OAuth2 token and add it to the Authorization header.
func (d *Dispatcher) Ping(ctx context.Context, opts PingOptions) error {
	d.logger.Debugf("rules: %+v", d.rules)

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	if d.ff.CanAccessFeature(fflag.IpRules) && d.l.IpRules() {
		ctx = netjail.ContextWithRules(ctx, d.rules)
	}

	// Get ping methods from config
	var methods []string
	cfg, err := config.Get()
	if err != nil {
		d.logger.WithError(err).Warn("Failed to get config, using default ping methods")
		methods = []string{"HEAD", "GET", "POST"}
	} else {
		methods = cfg.Dispatcher.PingMethods
		if len(methods) == 0 {
			methods = []string{"HEAD", "GET", "POST"}
		}
	}

	var lastErr error
	for i, method := range methods {
		pingOpts := opts
		pingOpts.Method = method
		err := d.tryPingMethod(ctx, pingOpts)
		if err == nil {
			if i > 0 {
				d.logger.Infof("Ping succeeded with %s after %d attempts", method, i+1)
			}
			return nil
		}

		lastErr = err
		d.logger.Debugf("Ping failed with %s: %v", method, err)
	}

	d.logger.Warnf("All ping methods failed for %s", opts.Endpoint)
	return lastErr
}

func (d *Dispatcher) tryPingMethod(ctx context.Context, opts PingOptions) error {
	client := d.createClientWithMTLS(opts.MtlsCert)

	var body []byte
	var reqContentType string

	if opts.Method == "POST" && opts.ContentType != "" {
		testPayload := json.RawMessage(`{"test": "ping"}`)
		converter := getConverter(opts.ContentType)
		var err error
		body, err = converter.Convert(testPayload)
		if err != nil {
			return err
		}
		reqContentType = converter.ContentType()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(ctx, opts.Method, opts.Endpoint, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", defaultUserAgent())
	if reqContentType != "" {
		req.Header.Set("Content-Type", reqContentType)
	}

	// Add OAuth2 Authorization header if token getter is provided
	if opts.OAuth2TokenGetter != nil {
		authHeader, err := opts.OAuth2TokenGetter(ctx)
		if err != nil {
			return fmt.Errorf("failed to get OAuth2 token for ping: %w", err)
		}
		req.Header.Set("Authorization", authHeader)
	}

	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("%w: got status code %d", ErrNon2xxResponse, response.StatusCode)
	}

	return nil
}
