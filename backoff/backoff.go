package backoff

import (
	"math"
	"time"
)

type Type string

type Strategy struct {
	Type Type `json:"type" bson:"type"`

	Interval uint64 `json:"interval" bson:"interval"`

	PreviousAttempts uint64 `json:"attempts" bson:"attempts"`

	RetryLimit uint64 `json:"retryLimit" bson:"retry_limit"`
}

const (
	Default     Type = "default"
	Exponential Type = "exponential"
)

var TypeValues = map[string]Type{
	"default":     Default,
	"exponential": Exponential,
}

func IsValidType(t string) bool {
	_, ok := TypeValues[t]
	return ok
}

func GetDelay(s Strategy) time.Duration {

	interval := s.Interval
	prevAttempts := s.PreviousAttempts

	if s.Type == Exponential {
		// (1/2) * (2^attempts - 1)
		return time.Duration((math.Pow(2, float64(interval*prevAttempts))-1)/2) * time.Second
	}

	if prevAttempts < 1 {
		return 0
	}
	return (time.Duration(interval)) * time.Second
}
