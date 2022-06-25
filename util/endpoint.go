package util

import (
	"errors"
	"net/url"
	"strings"
)

func CleanEndpoint(s string) (string, error) {
	if IsStringEmpty(s) {
		return "", errors.New("please provide the endpoint url")
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}

	switch u.Scheme {
	case "http", "https":
	default:
		return "", errors.New("invalid endpoint scheme")
	}

	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1":
		return "", errors.New("cannot use localhost or 127.0.0.1")
	}

	return u.String(), nil
}
