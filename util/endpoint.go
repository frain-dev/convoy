package util

import (
	"errors"
	"net/url"
	"strings"

	"github.com/frain-dev/convoy"
)

func ParseMetadataFromEndpoints(endpoints []convoy.Endpoint) []convoy.EndpointMetadata {
	m := make([]convoy.EndpointMetadata, 0)
	for _, e := range endpoints {
		m = append(m, convoy.EndpointMetadata{
			UID:       e.UID,
			TargetURL: e.TargetURL,
			Sent:      false,
		})
	}
	return m
}

func CleanEndpoint(s string) (string, error) {
	if IsStringEmpty(s) {
		return "", errors.New("please provide the endpoint url")
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}

	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1":
		return "", errors.New("cannot use localhost or 127.0.0.1")
	}

	if u.Scheme != "https" {
		return "", errors.New("endpoint scheme  must be HTTPs only")
	}

	return strings.ToLower(u.String()), nil
}
