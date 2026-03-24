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
	"time"

	"github.com/frain-dev/convoy/config"
)

func ValidateEndpoint(s string, enforceSecure, customCA bool) (string, error) {
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
			Transport: &http.Transport{
				DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					dialer := &net.Dialer{}
					return tls.DialWithDialer(dialer, network, addr, tlsConfig)
				},
			},
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
	default:
		return "", errors.New("invalid endpoint scheme")
	}

	return u.String(), nil
}
