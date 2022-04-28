package util

import (
	"errors"
	"net/url"
	"strings"

	"github.com/frain-dev/convoy/datastore"
)

func ParseMetadataFromActiveEndpoints(endpoints []datastore.Endpoint) []datastore.EndpointMetadata {
	return parseMetadataFromEndpoints(endpoints, func(e datastore.Endpoint) bool {
		return e.Status == datastore.ActiveEndpointStatus
	})
}

func GetMetadataFromEndpoints(endpoints []datastore.Endpoint) []datastore.EndpointMetadata {
	return parseMetadataFromEndpoints(endpoints, func(e datastore.Endpoint) bool {
		return true
	})
}

func parseMetadataFromEndpoints(endpoints []datastore.Endpoint, filter func(e datastore.Endpoint) bool) []datastore.EndpointMetadata {
	m := make([]datastore.EndpointMetadata, 0)
	for _, e := range endpoints {
		if filter(e) {
			m = append(m, datastore.EndpointMetadata{
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

	return u.String(), nil
}
