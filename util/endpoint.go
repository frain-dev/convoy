package util

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

func ValidateEndpoint(s string, enforceSecure bool) (string, error) {
	if IsStringEmpty(s) {
		return "", errors.New("please provide the endpoint url")
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}

	switch u.Scheme {
	case "http":
		if enforceSecure {
			return "", errors.New("only https endpoints allowed")
		}
	case "https":
		client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return tls.Dial(network, addr, &tls.Config{MinVersion: tls.VersionTLS12})
			},
		}}

		_, err = client.Get(s)
		if err != nil {
			return "", fmt.Errorf("failed to ping tls endpoint: %v", err)
		}
	default:
		return "", errors.New("invalid endpoint scheme")
	}

	return u.String(), nil
}
