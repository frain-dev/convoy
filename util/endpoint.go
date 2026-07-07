package util

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/frain-dev/convoy/config"
)

// ValidateOutboundURL validates that a user-supplied outbound URL is well formed
// and does not target loopback, private, or link-local addresses. It performs no
// network I/O, so it is safe for URLs that only accept POST (e.g. Slack webhooks)
// and cannot be pinged. It returns the normalized URL.
//
// Note: this rejects IP-literal SSRF targets at write time. DNS names that
// resolve to private IPs are caught at dispatch time by the netjail dispatcher.
func ValidateOutboundURL(s string, enforceSecure bool) (string, error) {
	if IsStringEmpty(s) {
		return "", errors.New("please provide the endpoint url")
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}

	if u.Host == "" {
		return "", errors.New("endpoint url must include a valid host")
	}

	hostname := strings.ToLower(u.Hostname())
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == "0.0.0.0" {
		return "", errors.New("endpoint url must not point to localhost or loopback addresses")
	}

	if ip := net.ParseIP(hostname); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return "", errors.New("endpoint url must not point to private or reserved IP addresses")
		}
	}

	switch u.Scheme {
	case "http":
		if enforceSecure {
			return "", errors.New("only https endpoints allowed")
		}
	case "https":
	default:
		return "", errors.New("invalid endpoint scheme")
	}

	return u.String(), nil
}

func ValidateEndpoint(s string, enforceSecure, customCA bool) (string, error) {
	normalized, err := ValidateOutboundURL(s, enforceSecure)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}

	// Scheme, host, and IP-literal checks were done by ValidateOutboundURL. For
	// https, additionally ping to verify the TLS endpoint is reachable/valid.
	if u.Scheme == "https" {
		var tlsConfig *tls.Config
		if customCA {
			tlsConfig, err = config.GetCaCert()
			if err != nil {
				return "", fmt.Errorf("could not get tls config: %w", err)
			}
		}
		if tlsConfig == nil {
			tlsConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		}
		client := &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: otelhttp.NewTransport(&http.Transport{
				DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					dialer := &net.Dialer{}
					return tls.DialWithDialer(dialer, network, addr, tlsConfig)
				},
			}),
		}

		resp, getErr := client.Get(u.String())
		if getErr != nil {
			return "", fmt.Errorf("failed to ping tls endpoint: %v", getErr)
		}

		defer func(Body io.ReadCloser) {
			err = Body.Close()
			if err != nil {
				fmt.Println("failed to close response body")
			}
		}(resp.Body)
	}

	return u.String(), nil
}
