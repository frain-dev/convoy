package util

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strings"
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
		_, err = tls.Dial("tcp", u.Host, &tls.Config{MinVersion: tls.VersionTLS12})
		if err != nil {
			return "", fmt.Errorf("failed to ping tls endpoint: %v", err)
		}
	default:
		return "", errors.New("invalid endpoint scheme")
	}

	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1":
		return "", errors.New("cannot use localhost or 127.0.0.1")
	}

	return u.String(), nil
}
