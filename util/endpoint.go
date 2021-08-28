package util

import "github.com/hookcamp/hookcamp"

func ParseMetadataFromEndpoints(endpoints []hookcamp.Endpoint) []hookcamp.EndpointMetadata {
	m := make([]hookcamp.EndpointMetadata, 0)
	for _, e := range endpoints {
		m = append(m, hookcamp.EndpointMetadata{
			UID:       e.UID,
			TargetURL: e.TargetURL,
			Merged:    false,
		})
	}
	return m
}
