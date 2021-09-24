package util

import "github.com/frain-dev/convoy"

func ParseMetadataFromActiveEndpoints(endpoints []convoy.Endpoint) []convoy.EndpointMetadata {
	m := make([]convoy.EndpointMetadata, 0)
	for _, e := range endpoints {
		if !e.Disabled {
			m = append(m, convoy.EndpointMetadata{
				UID:       e.UID,
				TargetURL: e.TargetURL,
				Sent:      false,
			})
		}
	}
	return m
}
