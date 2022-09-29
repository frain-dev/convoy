package main

import "encoding/json"

type Scheme struct {
	Secret   []string
	Hash     string
	Encoding string
}

type Signature struct {
	Payload json.RawMessage

	// The order of this Schemes is a core part of this API.
	// We use the index as the version number. That is:
	// Index 0 = v0, Index 1 = v1
	Schemes   []Scheme
	Versioned bool
	Timestamp bool

	// Cached value
	computedValue string
}

func (s *Signature) ComputeHeaderValue() (string, error) {
	return "", nil
}
