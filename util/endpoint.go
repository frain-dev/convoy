package util

import (
	"errors"
	"net/url"
	"strings"

	"github.com/frain-dev/convoy"
)

func ParseMetadataFromActiveEndpoints(endpoints []convoy.Endpoint) []convoy.EndpointMetadata {
	return parseMetadataFromEndpoints(endpoints, func(e convoy.Endpoint) bool {
		return e.Status == convoy.ActiveEndpointStatus
	})
}

func GetMetadataFromEndpoints(endpoints []convoy.Endpoint) []convoy.EndpointMetadata {
	return parseMetadataFromEndpoints(endpoints, func(e convoy.Endpoint) bool {
		return true
	})
}

func parseMetadataFromEndpoints(endpoints []convoy.Endpoint, filter func(e convoy.Endpoint) bool) []convoy.EndpointMetadata {
	m := make([]convoy.EndpointMetadata, 0)
	for _, e := range endpoints {
		if filter(e) {
			m = append(m, convoy.EndpointMetadata{
				UID:       e.UID,
				TargetURL: e.TargetURL,
				Sent:      false,
			})
		}
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

	switch u.Scheme {
	case "http", "https":
	default:
		return "", errors.New("invalid endpoint scheme")
	}

	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1":
		return "", errors.New("cannot use localhost or 127.0.0.1")
	}

	return strings.ToLower(u.String()), nil
}
